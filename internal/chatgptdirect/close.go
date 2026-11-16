package chatgptdirect

import "io"

func closeReadBody(body io.Closer) {
	// Read/decode errors are checked at the call site. A read-only stream's
	// cleanup error cannot change an already consumed response or cause retries.
	if err := body.Close(); err != nil {
		return
	}
}
