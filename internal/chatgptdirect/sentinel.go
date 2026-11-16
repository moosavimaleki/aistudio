package chatgptdirect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tlsclient "github.com/bogdanfinn/tls-client"
)

type sentinelProofEngine interface {
	initialProof(context.Context) (string, error)
	enforcementEvidence(context.Context, string, string) (string, string, error)
}

// Each submission gets fresh prepare/evidence/finalize results. Requirements
// tokens and their evidence are never reused for another turn.
type sentinelClient struct {
	http        tlsclient.HttpClient
	cookieDir   string
	baseURL     string
	sessionFile string
	node        sentinelProofEngine
}

func newSentinelClient(http tlsclient.HttpClient, cookieDir, baseURL string) *sentinelClient {
	if baseURL == "" {
		baseURL = upstreamOrigin
	}
	return &sentinelClient{
		http: http, cookieDir: cookieDir, baseURL: baseURL,
		node: newSentinelNodeRunner(sentinelSidecarPath()),
	}
}

func sentinelSidecarPath() string {
	if path := os.Getenv("SENTINEL_SIDECAR_PATH"); path != "" {
		return path
	}
	for _, path := range []string{"assets/sentinel/sentinel-node-probe.js", "/app/sentinel/sentinel-node-probe.js"} {
		if _, err := os.Stat(path); err == nil {
			absolute, err := filepath.Abs(path)
			if err == nil {
				return absolute
			}
		}
	}
	return "/app/sentinel/sentinel-node-probe.js"
}

func (s *sentinelClient) authHeaders(ctx context.Context, browserID string) (map[string]string, error) {
	headers := map[string]string{}
	if s.sessionFile != "" {
		var err error
		headers, err = loadSavedHeaders(s.sessionFile)
		if err != nil {
			return nil, err
		}
		if !bearerExpired(headers["authorization"]) {
			return headers, nil
		}
	} else {
		cookie, err := readChatGPTCookie(s.cookieDir, browserID)
		if err != nil {
			return nil, err
		}
		headers["cookie"] = cookie
		headers["user-agent"] = sentinelUserAgent
		headers["accept-language"] = sentinelAcceptLanguage
		headers["oai-device-id"] = deriveDeviceID(cookie)
		headers["oai-language"] = "en-US"
		headers["origin"] = upstreamOrigin
		headers["referer"] = upstreamOrigin + "/"
	}
	session, err := s.getSession(ctx, headers["cookie"])
	if err != nil {
		return nil, err
	}
	headers["authorization"] = "Bearer " + session.AccessToken
	if headers["chatgpt-account-id"] == "" && session.AccountID != "" {
		headers["chatgpt-account-id"] = session.AccountID
	}
	return headers, nil
}

func (s *sentinelClient) prepare(ctx context.Context, browserID string) (Artifacts, error) {
	headers, err := s.authHeaders(ctx, browserID)
	if err != nil {
		return Artifacts{}, err
	}
	initial, err := s.node.initialProof(ctx)
	if err != nil {
		return Artifacts{}, err
	}
	prepare, err := s.postPrepare(ctx, headers, initial)
	if err != nil {
		return Artifacts{}, err
	}
	proof, turnstile, err := s.node.enforcementEvidence(ctx, string(prepare.raw), initial)
	if err != nil {
		return Artifacts{}, err
	}
	missingProof := prepare.ProofOfWork.Required && proof == ""
	missingVM := prepare.Turnstile.Required && turnstile == ""
	if missingProof || missingVM {
		return Artifacts{}, fmt.Errorf("sentinel SDK did not produce the required evidence")
	}
	token, err := s.postFinalize(ctx, headers, prepare, proof, turnstile)
	if err != nil {
		return Artifacts{}, err
	}
	headers["openai-sentinel-chat-requirements-token"] = token
	if proof != "" {
		headers["openai-sentinel-proof-token"] = proof
	}
	if turnstile != "" {
		headers["openai-sentinel-turnstile-token"] = turnstile
	}
	return Artifacts{
		Headers: headers, Cookies: headers["cookie"], UserAgent: headers["user-agent"], BrowserID: browserID,
		Context: BrowserContext{
			AcceptLanguage: headers["accept-language"], SecCHUA: headers["sec-ch-ua"],
			SecCHUAMobile: headers["sec-ch-ua-mobile"], SecCHUAPlatform: headers["sec-ch-ua-platform"],
		},
	}, nil
}
