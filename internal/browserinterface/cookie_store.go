package browserinterface

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chromedp/cdproto/network"
)

func persistCookieFile(path string, cookies []*network.Cookie) error {
	return persistDomainCookieFile(path, cookies, "google.com")
}

func persistDomainCookieFile(path string, cookies []*network.Cookie, hostname string) error {
	records := make([][]string, 0, len(cookies))
	for _, cookie := range cookies {
		domain := strings.ToLower(strings.TrimSpace(cookie.Domain))
		normalized := strings.TrimPrefix(domain, ".")
		if cookie.Name == "" || cookie.Value == "" || strings.HasPrefix(cookie.Name, "__Host-") {
			continue
		}
		if normalized != hostname && !strings.HasSuffix(normalized, "."+hostname) {
			continue
		}
		outputDomain := domain
		if cookie.HTTPOnly {
			outputDomain = "#HttpOnly_" + outputDomain
		}
		expires := int64(cookie.Expires)
		if expires < 0 || cookie.Session {
			expires = 0
		}
		records = append(records, []string{
			outputDomain,
			boolText(strings.HasPrefix(domain, ".")),
			cleanCookieField(cookie.Path, "/"),
			boolText(cookie.Secure),
			fmt.Sprint(expires),
			cleanCookieField(cookie.Name, ""),
			cleanCookieField(cookie.Value, ""),
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return strings.Join(records[i][:6], "\x00") < strings.Join(records[j][:6], "\x00")
	})
	return replaceCookieFile(path, records)
}

func replaceCookieFile(path string, records [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	output, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	temporary := output.Name()
	defer os.Remove(temporary)

	writer := bufio.NewWriter(output)
	_, err = writer.WriteString("# Netscape HTTP Cookie File\n")
	for _, record := range records {
		if err == nil {
			_, err = writer.WriteString(strings.Join(record, "\t") + "\n")
		}
	}
	if flushErr := writer.Flush(); err == nil {
		err = flushErr
	}
	if syncErr := output.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(temporary, mode); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func cleanCookieField(value, fallback string) string {
	value = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(value)
	if value == "" {
		return fallback
	}
	return value
}

func boolText(value bool) string {
	if value {
		return "TRUE"
	}
	return "FALSE"
}
