package browserinterface

import (
	"os"
	"strings"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

type CookieDiagnostics struct {
	Count         int
	AuthCount     int
	Revision      int64
	SourceCurrent bool
}

func (s *ChromeSession) CookieDiagnostics() CookieDiagnostics {
	s.stateMu.RLock()
	cookies := append([]aistudio.CookieRecord{}, s.cookies...)
	if len(cookies) == 0 {
		cookies = parseCookieHeader(s.spec.CookieHeader)
	}
	file := s.spec.CookieFile
	s.stateMu.RUnlock()

	diagnostics := CookieDiagnostics{Count: len(cookies)}
	for _, cookie := range cookies {
		if strings.Contains(cookie.Name, "APISID") {
			diagnostics.AuthCount++
		}
	}
	if info, err := os.Stat(file); err == nil {
		diagnostics.Revision = info.ModTime().UnixMilli()
		diagnostics.SourceCurrent = true
	}
	return diagnostics
}
