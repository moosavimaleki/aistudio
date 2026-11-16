package chatgptdirect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// readChatGPTCookie returns the chatgpt.com Cookie header for browserID, read
// from the Netscape cookie files under CHATGPT_COOKIE_DIR. Files are ordered by
// name; browserID selects the slot the browser path uses (default → first file,
// a second-slot id like "browser2" → second file when present).
func readChatGPTCookie(dir, browserID string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("CHATGPT_COOKIE_DIR is not set")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read chatgpt cookie dir %s: %w", dir, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) == 0 {
		return "", fmt.Errorf("no cookie files found in %s", dir)
	}
	text, err := os.ReadFile(filepath.Join(dir, files[cookieIndexFor(browserID, len(files))]))
	if err != nil {
		return "", fmt.Errorf("read chatgpt cookie file %s: %w", files[0], err)
	}
	return parseChatGPTCookies(string(text), "chatgpt.com")
}

// cookieIndexFor maps a browser id to a cookie-file slot. The default and any
// unrecognised id use the first file; ids carrying a second-slot marker
// ("browser2", "2", …) use the second file when present.
func cookieIndexFor(browserID string, n int) int {
	if n < 2 {
		return 0
	}
	lower := strings.ToLower(browserID)
	if strings.Contains(lower, "2") || strings.Contains(lower, "second") {
		return 1
	}
	return 0
}

// parseChatGPTCookies reads a Netscape cookie file and returns the cookie header
// (name=value; …) for hostname, keeping the first value seen per cookie name.
func parseChatGPTCookies(text, hostname string) (string, error) {
	seen := make(map[string]bool)
	var parts []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 || !cookieDomainMatches(fields[0], hostname) {
			continue
		}
		name := fields[5]
		value := strings.Join(fields[6:], "\t")
		if seen[name] {
			continue
		}
		seen[name] = true
		parts = append(parts, name+"="+value)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no cookies for %s found", hostname)
	}
	return strings.Join(parts, "; "), nil
}

func cookieDomainMatches(domain, hostname string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == domain || strings.HasSuffix(hostname, "."+domain)
}
