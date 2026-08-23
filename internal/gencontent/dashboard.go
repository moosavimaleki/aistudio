package gencontent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hamed/aistudio-api/internal/metrics"
)

type Dashboard struct {
	store         *metrics.Store
	pool          *Pool
	browserOrigin string
	client        *http.Client
}

type browserHealth struct {
	Backend   string           `json:"backend"`
	Connected bool             `json:"connected"`
	Browsers  []browserSession `json:"browsers"`
}

type browserSession struct {
	AuthUser            string  `json:"authUser"`
	BrowserID           string  `json:"browserId"`
	CookieCount         int     `json:"cookieCount"`
	AuthCookieCount     int     `json:"authCookieCount"`
	CookieRevision      int64   `json:"cookieRevision"`
	CookieSourceCurrent bool    `json:"cookieSourceCurrent"`
	Connected           bool    `json:"connected"`
	HeartbeatAgeSeconds float64 `json:"heartbeatAgeSeconds"`
	PendingJobs         int     `json:"pendingJobs"`
	Ready               bool    `json:"ready"`
	SessionState        string  `json:"sessionState"`
	WarmError           string  `json:"warmError"`
}

func NewDashboard(store *metrics.Store, pool *Pool, browserOrigin string) *Dashboard {
	if browserOrigin == "" {
		browserOrigin = "http://127.0.0.1:3345"
	}
	return &Dashboard{
		store:         store,
		pool:          pool,
		browserOrigin: browserOrigin,
		client:        &http.Client{Timeout: 3 * time.Second},
	}
}

func (d *Dashboard) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", d.page)
	mux.HandleFunc("/dashboard/data", d.data)
	mux.HandleFunc("/dashboard/assets/style.css", d.style)
	mux.HandleFunc("/dashboard/assets/app.js", d.script)
}

func (d *Dashboard) page(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" || request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writeAsset(writer, "text/html; charset=utf-8", dashboardHTML)
}

func (d *Dashboard) data(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	window := dashboardWindow(request.URL.Query().Get("window"))
	browser, browserError := d.readBrowserHealth(request)
	metricsWindow := d.store.Read(request.Context(), window)
	writeJSON(writer, http.StatusOK, dashboardSnapshot(
		metricsWindow,
		window,
		d.pool.Stats(request.Context()),
		browser,
		browserError,
	))
}

func (d *Dashboard) style(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeAsset(writer, "text/css; charset=utf-8", dashboardStyle)
}

func (d *Dashboard) script(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeAsset(writer, "application/javascript; charset=utf-8", dashboardScript)
}

func writeAsset(writer http.ResponseWriter, contentType string, asset []byte) {
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(asset)
}

func (d *Dashboard) readBrowserHealth(request *http.Request) (browserHealth, string) {
	url := d.browserOrigin + "/health"
	probe, err := http.NewRequestWithContext(request.Context(), http.MethodGet, url, nil)
	if err != nil {
		return browserHealth{}, err.Error()
	}
	response, err := d.client.Do(probe)
	if err != nil {
		return browserHealth{}, err.Error()
	}
	defer response.Body.Close()
	var health browserHealth
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		return browserHealth{}, err.Error()
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return health, fmt.Sprintf("browser health returned HTTP %d", response.StatusCode)
	}
	return health, ""
}

func dashboardWindow(value string) int {
	minutes, err := strconv.Atoi(value)
	if err != nil || minutes < 1 {
		return 60
	}
	return minutes
}
