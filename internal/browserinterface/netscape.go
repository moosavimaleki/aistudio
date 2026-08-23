package browserinterface

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type netscapeCookie struct {
	Domain, Path, Name, Value string
	Secure, HTTPOnly          bool
}

func readNetscapeCookies(path, hostname string) ([]netscapeCookie, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []netscapeCookie
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 || !cookieDomainMatches(fields[0], hostname) {
			continue
		}
		result = append(result, netscapeCookie{
			Domain: fields[0], Path: cleanCookieField(fields[2], "/"),
			Secure: strings.EqualFold(fields[3], "TRUE"), HTTPOnly: httpOnly,
			Name: fields[5], Value: strings.Join(fields[6:], "\t"),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("cookie file contains no cookies for %s", hostname)
	}
	return result, nil
}

func cookieDomainMatches(domain, hostname string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == domain || strings.HasSuffix(hostname, "."+domain)
}
