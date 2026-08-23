package gencontent

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/hamed/aistudio-api/internal/metrics"
)

var dashboardLatencyBuckets = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

type dashboardResponse struct {
	GeneratedAt   time.Time          `json:"generatedAt"`
	WindowMinutes int                `json:"windowMinutes"`
	Summary       dashboardSummary   `json:"summary"`
	Pool          map[string]any     `json:"pool"`
	Profiles      []dashboardProfile `json:"profiles"`
	Models        []dashboardModel   `json:"models"`
	ErrorPhases   map[string]float64 `json:"errorPhases"`
	ErrorStatuses map[string]float64 `json:"errorStatuses"`
	Series        []dashboardSeries  `json:"series"`
	Events        []map[string]any   `json:"events"`
	BrowserError  string             `json:"browser_error,omitempty"`
}

type dashboardSummary struct {
	Requests        int     `json:"requests"`
	Success         int     `json:"success"`
	Errors          int     `json:"errors"`
	SuccessRate     float64 `json:"successRate"`
	Inflight        int64   `json:"inflight"`
	RPS             float64 `json:"rps"`
	LatencyP50      int     `json:"latencyP50"`
	LatencyP95      int     `json:"latencyP95"`
	TokenSuccess    int     `json:"tokenSuccess"`
	TokenErrors     int     `json:"tokenErrors"`
	TokenLatencyP95 int     `json:"tokenLatencyP95"`
	CookieRotations int     `json:"cookieRotations"`
	Attachments     int     `json:"attachments"`
}

type dashboardProfile struct {
	BrowserID           string  `json:"browser_id"`
	AuthUser            string  `json:"auth_user"`
	CookieCount         int     `json:"cookie_count"`
	AuthCookieCount     int     `json:"auth_cookie_count"`
	CookieRevision      int64   `json:"cookie_revision"`
	CookieSourceCurrent bool    `json:"cookie_source_current"`
	SessionState        string  `json:"session_state"`
	Connected           bool    `json:"connected"`
	Ready               bool    `json:"ready"`
	HeartbeatAgeSeconds float64 `json:"heartbeat_age_seconds"`
	WarmError           string  `json:"warm_error"`
}

type dashboardModel struct {
	Model       string  `json:"model"`
	Requests    int     `json:"requests"`
	Success     int     `json:"success"`
	Errors      int     `json:"errors"`
	SuccessRate float64 `json:"successRate"`
	P50         int     `json:"p50"`
	P95         int     `json:"p95"`
	Empty       int     `json:"empty"`
}

type dashboardSeries struct {
	Timestamp int64 `json:"timestamp"`
	Requests  int   `json:"requests"`
	Success   int   `json:"success"`
	Errors    int   `json:"errors"`
	P50       int   `json:"p50"`
	P95       int   `json:"p95"`
}

func dashboardSnapshot(
	window metrics.Window,
	minutes int,
	pool map[string]any,
	browser browserHealth,
	browserError string,
) dashboardResponse {
	requests := responseTotal(window.Aggregate, generationResponse)
	success := responseTotal(window.Aggregate, successfulGenerationResponse)
	errors := responseTotal(window.Aggregate, failedGenerationResponse)
	preparedPool := dashboardPool(pool)

	return dashboardResponse{
		GeneratedAt:   time.Now().UTC(),
		WindowMinutes: minutes,
		Summary: dashboardSummary{
			Requests:        int(requests),
			Success:         int(success),
			Errors:          int(errors),
			SuccessRate:     percentage(success, success+errors),
			Inflight:        window.Inflight,
			RPS:             round(requests / float64(minutes*60)),
			LatencyP50:      latencyPercentile(window.Aggregate, 0.50, generationDuration),
			LatencyP95:      latencyPercentile(window.Aggregate, 0.95, generationDuration),
			TokenSuccess:    int(responseTotal(window.Aggregate, successfulTokenResponse)),
			TokenErrors:     int(responseTotal(window.Aggregate, failedTokenResponse)),
			TokenLatencyP95: latencyPercentile(window.Aggregate, 0.95, tokenDuration),
		},
		Pool:          preparedPool,
		Profiles:      dashboardProfiles(browser.Browsers),
		Models:        dashboardModels(window.Aggregate),
		ErrorPhases:   errorPhases(errors),
		ErrorStatuses: errorStatuses(window.Aggregate),
		Series:        dashboardSeriesFor(window.Minutes),
		Events:        dashboardEvents(window.Events),
		BrowserError:  browserError,
	}
}

func dashboardPool(pool map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range pool {
		result[key] = value
	}
	result["tabs"] = []any{}
	return result
}

func dashboardProfiles(sessions []browserSession) []dashboardProfile {
	profiles := make([]dashboardProfile, 0, len(sessions))
	for _, session := range sessions {
		profiles = append(profiles, dashboardProfile{
			BrowserID:           session.BrowserID,
			AuthUser:            session.AuthUser,
			CookieCount:         session.CookieCount,
			AuthCookieCount:     session.AuthCookieCount,
			CookieRevision:      session.CookieRevision,
			CookieSourceCurrent: session.CookieSourceCurrent,
			SessionState:        session.SessionState,
			Connected:           session.Connected,
			Ready:               session.Ready,
			HeartbeatAgeSeconds: session.HeartbeatAgeSeconds,
			WarmError:           session.WarmError,
		})
	}
	return profiles
}

func dashboardModels(values map[string]float64) []dashboardModel {
	models := map[string]bool{}
	for field := range values {
		metric, labels := metrics.ParseField(field)
		if metric == "http.response" && generationRoute(labels["route"]) && labels["model"] != "" {
			models[labels["model"]] = true
		}
	}
	rows := make([]dashboardModel, 0, len(models))
	for model := range models {
		match := func(labels map[string]string) bool {
			return generationRoute(labels["route"]) && labels["model"] == model
		}
		requests := responseTotal(values, match)
		success := responseTotal(values, func(labels map[string]string) bool {
			return match(labels) && successfulStatus(labels["status"])
		})
		errors := responseTotal(values, func(labels map[string]string) bool {
			return match(labels) && failedStatus(labels["status"])
		})
		rows = append(rows, dashboardModel{
			Model:       model,
			Requests:    int(requests),
			Success:     int(success),
			Errors:      int(errors),
			SuccessRate: percentage(success, success+errors),
			P50:         latencyPercentile(values, 0.50, match),
			P95:         latencyPercentile(values, 0.95, match),
		})
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].Requests > rows[right].Requests })
	return rows
}

func dashboardSeriesFor(minutes []map[string]float64) []dashboardSeries {
	series := make([]dashboardSeries, 0, len(minutes))
	first := time.Now().Unix()/60 - int64(len(minutes)) + 1
	for index, values := range minutes {
		series = append(series, dashboardSeries{
			Timestamp: (first + int64(index)) * 60000,
			Requests:  int(responseTotal(values, generationResponse)),
			Success:   int(responseTotal(values, successfulGenerationResponse)),
			Errors:    int(responseTotal(values, failedGenerationResponse)),
			P50:       latencyPercentile(values, 0.50, generationDuration),
			P95:       latencyPercentile(values, 0.95, generationDuration),
		})
	}
	return series
}

func dashboardEvents(events []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(events))
	return append(result, events...)
}

func errorPhases(errors float64) map[string]float64 {
	result := map[string]float64{}
	if errors > 0 {
		result["GenerateContent"] = errors
	}
	return result
}

func errorStatuses(values map[string]float64) map[string]float64 {
	result := map[string]float64{}
	for field, value := range values {
		metric, labels := metrics.ParseField(field)
		if metric == "http.response" && generationRoute(labels["route"]) && failedStatus(labels["status"]) {
			result[labels["status"]] += value
		}
	}
	return result
}

func responseTotal(values map[string]float64, match func(map[string]string) bool) float64 {
	return metricTotal(values, "http.response", match)
}

func latencyPercentile(values map[string]float64, ratio float64, match func(map[string]string) bool) int {
	count := metricTotal(values, "http.duration.count", match)
	if count == 0 {
		return 0
	}
	for _, boundary := range dashboardLatencyBuckets {
		name := "http.duration.le_" + strconv.FormatFloat(boundary, 'f', -1, 64)
		if metricTotal(values, name, match) >= count*ratio {
			return int(boundary)
		}
	}
	return int(dashboardLatencyBuckets[len(dashboardLatencyBuckets)-1])
}

func metricTotal(values map[string]float64, name string, match func(map[string]string) bool) float64 {
	total := 0.0
	for field, value := range values {
		metric, labels := metrics.ParseField(field)
		if metric == name && match(labels) {
			total += value
		}
	}
	return total
}

func generationResponse(labels map[string]string) bool { return generationRoute(labels["route"]) }
func successfulGenerationResponse(labels map[string]string) bool {
	return generationResponse(labels) && successfulStatus(labels["status"])
}
func failedGenerationResponse(labels map[string]string) bool {
	return generationResponse(labels) && failedStatus(labels["status"])
}
func successfulTokenResponse(labels map[string]string) bool {
	return labels["route"] == "getToken" && successfulStatus(labels["status"])
}
func failedTokenResponse(labels map[string]string) bool {
	return labels["route"] == "getToken" && failedStatus(labels["status"])
}
func generationDuration(labels map[string]string) bool { return generationRoute(labels["route"]) }
func tokenDuration(labels map[string]string) bool      { return labels["route"] == "getToken" }
func generationRoute(route string) bool {
	return route == "generateContent" || route == "streamGenerateContent" || route == "legacyGenerateContent"
}
func successfulStatus(value string) bool {
	status, _ := strconv.Atoi(value)
	return status >= httpStatusOK && status < httpStatusMultipleChoices
}
func failedStatus(value string) bool {
	status, _ := strconv.Atoi(value)
	return status >= httpStatusBadRequest
}
func percentage(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return round(value * 100 / total)
}
func round(value float64) float64 { return math.Round(value*1000) / 1000 }

const (
	httpStatusOK              = 200
	httpStatusMultipleChoices = 300
	httpStatusBadRequest      = 400
)
