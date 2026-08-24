package browserinterface

import (
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	enabledChatGPT, err := parseBrowserIDSet(os.Getenv("CHATGPT_ENABLED_BROWSER_IDS"))
	if err != nil {
		return Config{}, err
	}
	filterChatGPT := len(enabledChatGPT) > 0
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
	chatGPTIDs := make([]string, 0, len(chatGPTFiles))
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
		if !browserIDEnabled(filterChatGPT, enabledChatGPT, id) {
			continue
		}
		if err := validateBrowserID(id, result.Browsers); err != nil {
			return Config{}, err
		}
		result.Browsers = append(result.Browsers, BrowserSpec{
			ID: id, Provider: ProviderChatGPT, CookieFile: file, ChatGPTCookieFile: file,
		})
		chatGPTIDs = append(chatGPTIDs, id)
		delete(enabledChatGPT, id)
	}
	if len(enabledChatGPT) > 0 {
		return Config{}, fmt.Errorf("unknown CHATGPT_ENABLED_BROWSER_IDS: %s", joinBrowserIDs(enabledChatGPT))
	}
	result.DefaultID = os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID")
	if result.DefaultID == "" {
		result.DefaultID = result.Browsers[0].ID
	}
	if len(chatGPTIDs) > 0 {
		result.ChatGPTDefaultID = os.Getenv("CHATGPT_DEFAULT_BROWSER_ID")
		if result.ChatGPTDefaultID == "" {
			result.ChatGPTDefaultID = chatGPTIDs[0]
		}
		if !containsBrowserID(chatGPTIDs, result.ChatGPTDefaultID) {
			return Config{}, fmt.Errorf("CHATGPT_DEFAULT_BROWSER_ID is not enabled: %s", result.ChatGPTDefaultID)
		}
	}
	return result, nil
}

func parseBrowserIDSet(raw string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(id) {
			return nil, fmt.Errorf("invalid CHATGPT_ENABLED_BROWSER_IDS value: %s", id)
		}
		result[id] = true
	}
	return result, nil
}

func browserIDEnabled(filter bool, enabled map[string]bool, id string) bool {
	return !filter || enabled[id]
}

func joinBrowserIDs(values map[string]bool) string {
	result := make([]string, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Strings(result)
	return strings.Join(result, ",")
}

func containsBrowserID(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
