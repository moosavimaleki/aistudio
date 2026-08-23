package browserinterface

import (
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ProviderAIStudio = "aistudio"
	ProviderChatGPT  = "chatgpt"
)

type BrowserSpec struct {
	ID, Provider, AuthUser, CookieHeader, CookieFile string
	ChatGPTCookieFile                                string
}
type Config struct {
	Browsers         []BrowserSpec
	DefaultID        string
	ChatGPTDefaultID string
	CDPBasePort      int
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
	chatGPTFiles, err := discoverOptionalCookieFiles(os.Getenv("CHATGPT_COOKIE_DIR"))
	if err != nil {
		return Config{}, err
	}
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
		result.Browsers = append(result.Browsers, BrowserSpec{
			ID: id, Provider: ProviderAIStudio,
			AuthUser:     defaultAuthUser(settings.Values["AISTUDIO_AUTH_USER"+suffix], "0"),
			CookieHeader: header, CookieFile: file,
		})
	}
	for index, file := range chatGPTFiles {
		suffix := ""
		id := "chatgpt"
		if index > 0 {
			suffix = fmt.Sprintf("%d", index+1)
			id = "chatgpt" + suffix
		}
		if configured := os.Getenv("CHATGPT_BROWSER_ID" + suffix); configured != "" {
			id = configured
		}
		if err := validateBrowserID(id, result.Browsers); err != nil {
			return Config{}, err
		}
		result.Browsers = append(result.Browsers, BrowserSpec{
			ID: id, Provider: ProviderChatGPT, CookieFile: file, ChatGPTCookieFile: file,
		})
	}
	result.DefaultID = os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID")
	if result.DefaultID == "" {
		result.DefaultID = result.Browsers[0].ID
	}
	if len(chatGPTFiles) > 0 {
		result.ChatGPTDefaultID = os.Getenv("CHATGPT_DEFAULT_BROWSER_ID")
		if result.ChatGPTDefaultID == "" {
			result.ChatGPTDefaultID = "chatgpt"
		}
	}
	return result, nil
}

func discoverOptionalCookieFiles(directory string) ([]string, error) {
	if directory == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	found := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}
	return aistudio.DiscoverCookieFiles(directory)
}

func validateBrowserID(id string, existing []BrowserSpec) error {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(id) {
		return fmt.Errorf("invalid browserId: %s", id)
	}
	for _, spec := range existing {
		if spec.ID == id {
			return fmt.Errorf("duplicate browserId: %s", id)
		}
	}
	return nil
}
func defaultAuthUser(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
