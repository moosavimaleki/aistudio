package chatgptdirect

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

func TestTransportValidatesServerCertificate(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("untrusted server received a request")
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	client, err := newBrowserHTTP("")
	if err != nil {
		t.Fatal(err)
	}
	request, err := fhttp.NewRequest(fhttp.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err == nil {
		closeReadBody(response.Body)
		t.Fatal("transport accepted an untrusted certificate")
	}
}

func TestTransportReturnsRedirectWithoutFollowing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Error("transport followed the redirect")
		}
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer server.Close()
	client, err := newBrowserHTTP("")
	if err != nil {
		t.Fatal(err)
	}
	request, err := fhttp.NewRequest(fhttp.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeReadBody(response.Body)
	if response.StatusCode != http.StatusFound {
		t.Fatalf("redirect status = %d", response.StatusCode)
	}
}
