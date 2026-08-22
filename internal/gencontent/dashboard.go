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

type dashboardData struct {
	GeneratedAt  time.Time      `json:"generatedAt"`
	Pool         map[string]any `json:"pool"`
	Browser      browserHealth  `json:"browser"`
	BrowserError string         `json:"browserError,omitempty"`
	Metrics      metrics.Window `json:"metrics"`
}

type browserHealth struct {
	Backend   string           `json:"backend"`
	Connected bool             `json:"connected"`
	Browsers  []browserSession `json:"browsers"`
}

type browserSession struct {
	AuthUser            string  `json:"authUser"`
	BrowserID           string  `json:"browserId"`
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
}

func (d *Dashboard) page(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" || request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(dashboardHTML))
}

func (d *Dashboard) data(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	window := dashboardWindow(request.URL.Query().Get("window"))
	browser, browserError := d.readBrowserHealth(request)
	writeJSON(writer, http.StatusOK, dashboardData{
		GeneratedAt:  time.Now().UTC(),
		Pool:         d.pool.Stats(request.Context()),
		Browser:      browser,
		BrowserError: browserError,
		Metrics:      d.store.Read(request.Context(), window),
	})
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
