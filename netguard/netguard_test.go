package netguard

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckURLRejectsUnsupportedSchemes(t *testing.T) {
	for _, url := range []string{"", "ftp://example.com/data", "file:///etc/passwd", "http://"} {
		if err := CheckURL(url); err == nil {
			t.Errorf("CheckURL(%q) = nil, want error", url)
		}
	}
	for _, url := range []string{"http://example.com/data", "https://example.com/api?x=1"} {
		if err := CheckURL(url); err != nil {
			t.Errorf("CheckURL(%q) = %v, want nil", url, err)
		}
	}
}

func TestClientBlocksLoopbackAndPrivateTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{}")
	}))
	defer server.Close()

	client := NewClient(Options{Timeout: 5 * time.Second})
	resp, err := client.Get(server.URL) // httptest listens on 127.0.0.1
	if err == nil {
		resp.Body.Close()
		t.Fatalf("request to loopback test server succeeded; want SSRF block")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error does not mention block: %v", err)
	}
}

func TestClientAllowsPrivateWhenOptedIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{}")
	}))
	defer server.Close()

	client := NewClient(Options{Timeout: 5 * time.Second, AllowPrivate: true})
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request with AllowPrivate failed: %v", err)
	}
	resp.Body.Close()
}

func TestReadLimitedEnforcesCap(t *testing.T) {
	big := strings.Repeat("x", 1024)
	if _, err := ReadLimited(strings.NewReader(big), 128, "test"); err == nil {
		t.Fatalf("ReadLimited accepted body over the cap")
	}
	got, err := ReadLimited(strings.NewReader(big), 2048, "test")
	if err != nil {
		t.Fatalf("ReadLimited within cap failed: %v", err)
	}
	if len(got) != 1024 {
		t.Fatalf("ReadLimited returned %d bytes, want 1024", len(got))
	}
}
