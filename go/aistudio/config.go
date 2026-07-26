package aistudio

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Settings struct {
	EnvFile         string
	Values          map[string]string
	CookieHeader    string
	OriginURL       string
	Model           string
	TokenFactoryURL string
	WAAAPIKey       string
	ProxyURL        string
	AuthUser        string
	BrowserID       string
}

func ParseEnv(text string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		name, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		if name != "" {
			values[name] = value
		}
	}
	return values
}

func DiscoverCookieFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	sort.Slice(paths, func(i, j int) bool { return naturalLess(filepath.Base(paths[i]), filepath.Base(paths[j])) })
	if len(paths) == 0 {
		return nil, fmt.Errorf("no cookie files found in %s", directory)
	}
	return paths, nil
}

func naturalLess(left, right string) bool {
	for left != "" && right != "" {
		ld, rd := unicode.IsDigit(rune(left[0])), unicode.IsDigit(rune(right[0]))
		if ld && rd {
			li, ri := 0, 0
			for li < len(left) && left[li] >= '0' && left[li] <= '9' {
				li++
			}
			for ri < len(right) && right[ri] >= '0' && right[ri] <= '9' {
				ri++
			}
			ln, _ := strconv.Atoi(left[:li])
			rn, _ := strconv.Atoi(right[:ri])
			if ln != rn {
				return ln < rn
			}
			left, right = left[li:], right[ri:]
			continue
		}
		if left[0] != right[0] {
			return left[0] < right[0]
		}
		left, right = left[1:], right[1:]
	}
	return left == "" && right != ""
}

func ParseNetscapeCookieHeader(text, hostname string) (string, error) {
	if hostname == "" {
		return "", fmt.Errorf("cookie hostname is required")
	}
	var pairs []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		domain := strings.TrimPrefix(strings.ToLower(fields[0]), ".")
		if hostname != domain && !strings.HasSuffix(hostname, "."+domain) {
			continue
		}
		if fields[5] != "" && fields[6] != "" {
			pairs = append(pairs, fields[5]+"="+strings.Join(fields[6:], "\t"))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("cookie file contains no cookies for %s", hostname)
	}
	return strings.Join(pairs, "; "), nil
}

func LoadSettings(envFile string) (Settings, error) {
	if envFile == "" {
		envFile = os.Getenv("AISTUDIO_ENV_FILE")
	}
	if envFile == "" {
		envFile = ".env"
	}
	envFile, _ = filepath.Abs(envFile)
	text, _ := os.ReadFile(envFile)
	values := ParseEnv(string(text))
	for _, item := range os.Environ() {
		pair := strings.SplitN(item, "=", 2)
		if strings.HasPrefix(pair[0], "AISTUDIO_") || pair[0] == "TOKEN_FACTORY_URL" {
			values[pair[0]] = pair[1]
		}
	}
	cookieDir := values["AISTUDIO_COOKIE_DIR"]
	if cookieDir == "" {
		cookieDir = filepath.Join(filepath.Dir(envFile), "COOKIES")
	}
	if !filepath.IsAbs(cookieDir) {
		cookieDir = filepath.Join(filepath.Dir(envFile), cookieDir)
	}
	cookieDir, _ = filepath.Abs(cookieDir)
	values["AISTUDIO_COOKIE_DIR"] = cookieDir
	cookieFiles, err := DiscoverCookieFiles(cookieDir)
	if err != nil {
		return Settings{}, err
	}
	cookieText, err := os.ReadFile(cookieFiles[0])
	if err != nil {
		return Settings{}, err
	}
	upstream, err := LoadUpstream()
	if err != nil {
		return Settings{}, err
	}
	origin := mustValue(upstream.AIStudio, "origin")
	parsed, err := url.Parse(origin)
	if err != nil {
		return Settings{}, err
	}
	cookieHeader, err := ParseNetscapeCookieHeader(string(cookieText), parsed.Hostname())
	if err != nil {
		return Settings{}, err
	}
	return Settings{EnvFile: envFile, Values: values, CookieHeader: cookieHeader, OriginURL: origin, Model: values["AISTUDIO_MODEL"], TokenFactoryURL: values["TOKEN_FACTORY_URL"], WAAAPIKey: mustValue(upstream.Opaque, "waa_api_key"), ProxyURL: valueOr(values["AISTUDIO_PROXY_URL"], "http://127.0.0.1:10808"), AuthUser: valueOr(values["AISTUDIO_AUTH_USER"], "0"), BrowserID: values["AISTUDIO_BROWSER_ID"]}, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
