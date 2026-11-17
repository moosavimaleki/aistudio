package chatgptdirect

import "strings"

// blackBoxEnvironment forwards runtime configuration without application secrets.
func blackBoxEnvironment(environ []string) []string {
	var result []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR",
			"TMPDIR", "TMP", "TEMP", "XDG_CACHE_HOME", "LANG", "TZ",
			"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
			"http_proxy", "https_proxy", "all_proxy", "no_proxy",
			"NODE_USE_ENV_PROXY", "NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR",
			"CHATGPT_BLACK_BOX_CACHE_TTL_MS":
			result = append(result, entry)
		default:
			if strings.HasPrefix(name, "LC_") {
				result = append(result, entry)
			}
		}
	}
	return result
}
