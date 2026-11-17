package chatgptdirect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/hamed/aistudio-api/internal/metrics"
)

// blackBoxRuntime treats SDK calculations as opaque per-request data.
type blackBoxRuntime interface {
	initialize(context.Context) (string, error)
	evaluate(context.Context, string, string) (string, string, error)
}

// Each submission gets fresh prepare/evidence/finalize results. Requirements
// tokens and their evidence are never reused for another turn.
type blackBoxClient struct {
	http        tlsclient.HttpClient
	cookieDir   string
	baseURL     string
	sessionFile string
	runtime     blackBoxRuntime
}

func newBlackBoxClient(http tlsclient.HttpClient, cookieDir, baseURL string) *blackBoxClient {
	if baseURL == "" {
		baseURL = upstreamOrigin
	}
	return &blackBoxClient{
		http: http, cookieDir: cookieDir, baseURL: baseURL,
		runtime: newBlackBoxRunner(blackBoxScriptPath()),
	}
}

func blackBoxScriptPath() string {
	if path := os.Getenv("CHATGPT_BLACK_BOX_SCRIPT"); path != "" {
		return path
	}
	for _, path := range []string{"assets/blackbox/runner.cjs", "/app/blackbox/runner.cjs"} {
		if _, err := os.Stat(path); err == nil {
			absolute, err := filepath.Abs(path)
			if err == nil {
				return absolute
			}
		}
	}
	return "/app/blackbox/runner.cjs"
}

func (s *blackBoxClient) authHeaders(ctx context.Context, browserID string) (map[string]string, error) {
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
		headers["user-agent"] = clientUserAgent
		headers["accept-language"] = clientAcceptLanguage
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

func (s *blackBoxClient) prepare(ctx context.Context, browserID string) (Artifacts, error) {
	doneSession := metrics.Phase(ctx, "auth_session")
	headers, err := s.authHeaders(ctx, browserID)
	doneSession()
	if err != nil {
		return Artifacts{}, err
	}
	doneInitial := metrics.Phase(ctx, "black_box_initial")
	initial, err := s.runtime.initialize(ctx)
	doneInitial()
	if err != nil {
		return Artifacts{}, err
	}
	donePrepare := metrics.Phase(ctx, "black_box_prepare")
	prepare, err := s.postPrepare(ctx, headers, initial)
	donePrepare()
	if err != nil {
		return Artifacts{}, err
	}
	doneEvaluate := metrics.Phase(ctx, "black_box_evaluate")
	workToken, runtimeToken, err := s.runtime.evaluate(ctx, string(prepare.raw), initial)
	doneEvaluate()
	if err != nil {
		return Artifacts{}, err
	}
	missingWork := prepare.Work.Required && workToken == ""
	missingRuntime := prepare.Runtime.Required && runtimeToken == ""
	if missingWork || missingRuntime {
		return Artifacts{}, fmt.Errorf("black box runtime did not produce the required evidence")
	}
	doneFinalize := metrics.Phase(ctx, "black_box_finalize")
	token, err := s.postFinalize(ctx, headers, prepare, workToken, runtimeToken)
	doneFinalize()
	if err != nil {
		return Artifacts{}, err
	}
	headers[requirementsTokenHeader] = token
	if workToken != "" {
		headers[workTokenHeader] = workToken
	}
	if runtimeToken != "" {
		headers[runtimeTokenHeader] = runtimeToken
	}
	return Artifacts{
		Headers: headers, Cookies: headers["cookie"], UserAgent: headers["user-agent"], BrowserID: browserID,
		Context: BrowserContext{
			AcceptLanguage: headers["accept-language"], SecCHUA: headers["sec-ch-ua"],
			SecCHUAMobile: headers["sec-ch-ua-mobile"], SecCHUAPlatform: headers["sec-ch-ua-platform"],
		},
	}, nil
}
