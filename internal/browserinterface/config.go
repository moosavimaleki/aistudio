package browserinterface

import (
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"os"
	"regexp"
)

type BrowserSpec struct{ ID, AuthUser, CookieHeader, CookieFile string }
type Config struct {
	Browsers    []BrowserSpec
	DefaultID   string
	CDPBasePort int
}

func LoadConfig() (Config, error) {
	settings, err := aistudio.LoadSettings("")
	if err != nil {
		return Config{}, err
	}
	files, err := aistudio.DiscoverCookieFiles(settings.Values["AISTUDIO_COOKIE_DIR"])
	if err != nil {
		return Config{}, err
	}
	result := Config{CDPBasePort: 9223}
	if value := os.Getenv("CHROME_CDP_BASE_PORT"); value != "" {
		fmt.Sscanf(value, "%d", &result.CDPBasePort)
	}
	for index, file := range files {
		suffix := ""
		if index > 0 {
			suffix = fmt.Sprintf("%d", index+1)
		}
		id := settings.Values["AISTUDIO_BROWSER_ID"+suffix]
		if id == "" {
			if index == 0 {
				id = "default"
			} else {
				id = fmt.Sprintf("browser%d", index+1)
			}
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(id) {
			return Config{}, fmt.Errorf("invalid browserId: %s", id)
		}
		text, readErr := os.ReadFile(file)
		if readErr != nil {
			return Config{}, readErr
		}
		header, parseErr := aistudio.ParseNetscapeCookieHeader(string(text), "aistudio.google.com")
		if parseErr != nil {
			return Config{}, parseErr
		}
		result.Browsers = append(result.Browsers, BrowserSpec{ID: id, AuthUser: defaultAuthUser(settings.Values["AISTUDIO_AUTH_USER"+suffix], "0"), CookieHeader: header, CookieFile: file})
	}
	result.DefaultID = os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID")
	if result.DefaultID == "" {
		result.DefaultID = result.Browsers[0].ID
	}
	return result, nil
}
func defaultAuthUser(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
