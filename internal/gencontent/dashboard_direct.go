package gencontent

import (
	"os"
	"path/filepath"
	"strings"
)

type dashboardDirect struct {
	State           string `json:"state"`
	CredentialType  string `json:"credentialType"`
	CredentialFiles int    `json:"credentialFiles"`
}

// directDashboardState reports local configuration without reading credentials
// or contacting authentication, conversation history, or browser endpoints.
func directDashboardState() *dashboardDirect {
	state := &dashboardDirect{State: "UNAVAILABLE", CredentialType: "cookies"}
	if session := os.Getenv("CHATGPT_SESSION_FILE"); session != "" {
		state.CredentialType = "session"
		if file, err := os.Stat(session); err == nil && file.Mode().IsRegular() {
			state.CredentialFiles = 1
		}
	} else if entries, err := os.ReadDir(os.Getenv("CHATGPT_COOKIE_DIR")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
				state.CredentialFiles++
			}
		}
	}
	if state.CredentialFiles > 0 {
		state.State = "CONFIGURED"
	}
	return state
}
