package browserinterface

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type Server struct {
	fleet  *Fleet
	broker *Broker
	tokens *TokenService
}

func NewServer(fleet *Fleet, broker *Broker) *Server {
	return &Server{fleet: fleet, broker: broker, tokens: NewTokenService(broker, fleet)}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/bootstrap", s.bootstrap)
	mux.HandleFunc("/get-token", s.token)
	mux.HandleFunc("/internal/jobs/next", s.next)
	mux.HandleFunc("/internal/jobs/", s.complete)
	return mux
}
func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	status := http.StatusOK
	if !s.fleet.Healthy() {
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, map[string]any{
		"backend":     "container-extension",
		"connected":   s.fleet.Healthy(),
		"pendingJobs": 0,
		"browsers":    s.fleet.Status(),
	})
}
func (s *Server) bootstrap(writer http.ResponseWriter, request *http.Request) {
	body, ok := decodeBody(writer, request)
	if !ok {
		return
	}
	id, err := s.fleet.Resolve(stringValue(body["browserId"]))
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	spec, _ := s.fleet.Spec(id)
	cookies := stringValue(body["cookies"])
	if cookies == "" {
		cookies = spec.CookieHeader
	}
	authUser := stringValue(body["authUser"])
	if authUser == "" {
		authUser = spec.AuthUser
	}
	if SessionFingerprint(cookies, authUser) != SessionFingerprint(spec.CookieHeader, spec.AuthUser) {
		writeError(writer, http.StatusBadRequest, fmtError("Cookies do not belong to selected browserId: "+id))
		return
	}
	session, _ := s.fleet.Session(id)
	result, err := session.Prepare(cookies, authUser)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, err)
		return
	}
	result["browserId"] = id
	writeJSON(writer, http.StatusOK, result)
}
func (s *Server) token(writer http.ResponseWriter, request *http.Request) {
	body, ok := decodeBody(writer, request)
	if !ok {
		return
	}
	result, err := s.tokens.Create(body)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}
func (s *Server) next(writer http.ResponseWriter, request *http.Request) {
	id := request.URL.Query().Get("browserId")
	if id == "" {
		id = "default"
	}
	job := s.broker.Next(id)
	writer.Header().Set("Cache-Control", "no-store")
	if job == nil {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	payload := map[string]any{"id": job.ID}
	for name, value := range job.Payload {
		payload[name] = value
	}
	writeJSON(writer, http.StatusOK, payload)
}
func (s *Server) complete(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasSuffix(request.URL.Path, "/result") || request.Method != "POST" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	body, ok := decodeBody(writer, request)
	if !ok {
		return
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/internal/jobs/"), "/result"), "/")
	id := parts[0]
	browserID := request.URL.Query().Get("browserId")
	if browserID == "" {
		browserID = "default"
	}
	if !s.broker.Complete(id, browserID, body) {
		writeError(writer, http.StatusNotFound, fmtError("job is no longer pending"))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
func decodeBody(writer http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	defer request.Body.Close()
	body := map[string]any{}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return nil, false
	}
	return body, true
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		log.Printf("response encode: %v", err)
	}
}
func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]any{"error": err.Error()})
}
func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(fmt.Sprint(value), " "))
}

type staticError string

func (e staticError) Error() string { return string(e) }
func fmtError(value string) error   { return staticError(value) }
