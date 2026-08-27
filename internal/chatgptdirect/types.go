package chatgptdirect

import "strconv"

type Message struct {
	Role    string
	Content string
}

type Input struct {
	Model           string
	Messages        []Message
	BrowserID       string
	ConversationID  string
	ParentMessageID string
	IncludeHistory  bool
}

type Result struct {
	Text            string
	ConversationID  string
	ParentMessageID string
	BrowserID       string
	Model           string
	UpstreamStatus  int
	UpstreamPath    string
}

type BrowserContext struct {
	Timezone                      string  `json:"timezone"`
	TimezoneOffsetMin             int     `json:"timezoneOffsetMin"`
	AcceptLanguage                string  `json:"acceptLanguage"`
	SecCHUA                       string  `json:"secCHUA"`
	SecCHUAMobile                 string  `json:"secCHUAMobile"`
	SecCHUAPlatform               string  `json:"secCHUAPlatform"`
	IsDarkMode                    bool    `json:"isDarkMode"`
	TimeSinceLoaded               int     `json:"timeSinceLoaded"`
	PageHeight                    int     `json:"pageHeight"`
	PageWidth                     int     `json:"pageWidth"`
	PixelRatio                    float64 `json:"pixelRatio"`
	ScreenHeight                  int     `json:"screenHeight"`
	ScreenWidth                   int     `json:"screenWidth"`
	HasWebPushCapabilities        bool    `json:"hasWebPushCapabilities"`
	WebPushNotificationPermission string  `json:"webPushNotificationPermission"`
}

type Artifacts struct {
	Headers        map[string]string `json:"headers"`
	PrepareHeaders map[string]string `json:"prepareHeaders"`
	Cookies        string            `json:"cookies"`
	UserAgent      string            `json:"userAgent"`
	BrowserID      string            `json:"browserId"`
	UpstreamPath   string            `json:"upstreamPath"`
	Context        BrowserContext    `json:"context"`
}

type UpstreamError struct {
	Status      int
	Body        string
	BrowserID   string
	ContentType string
	CFMitigated string
}

type BridgeError struct {
	Status  int
	Message string
}

func (e *BridgeError) Error() string {
	return "ChatGPT browser preparation returned HTTP " + statusText(e.Status) + ": " + e.Message
}

func (e *UpstreamError) Error() string {
	message := "ChatGPT direct request returned HTTP " + statusText(e.Status)
	if e.CFMitigated != "" {
		message += " (cf-mitigated=" + e.CFMitigated + ")"
	}
	if e.Body != "" {
		message += ": " + e.Body
	}
	return message
}

func statusText(status int) string {
	return strconv.Itoa(status)
}
