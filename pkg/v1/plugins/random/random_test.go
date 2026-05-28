package random

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRandomPlugin_NameAndScreens(t *testing.T) {
	p := &RandomPlugin{ApiKey: "k"}
	if p.Name() != "random" {
		t.Errorf("Name = %q, want random", p.Name())
	}
	got := p.Screens()
	if len(got) != 1 || got[0] != "random" {
		t.Errorf("Screens = %v, want [random]", got)
	}
}

func withBaseURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
}

func TestGetImageURL_ReturnsRegular(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/photos/random") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("client_id") != "secret-key" {
			t.Errorf("client_id = %q, want secret-key", r.URL.Query().Get("client_id"))
		}
		w.Write([]byte(`{
			"urls": {
				"full":    "https://example.com/full.jpg",
				"regular": "https://example.com/regular.jpg",
				"small":   "https://example.com/small.jpg",
				"thumb":   "https://example.com/thumb.jpg"
			}
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	url, err := getImageURL("secret-key")
	if err != nil {
		t.Fatalf("getImageURL: %v", err)
	}
	if url != "https://example.com/regular.jpg" {
		t.Errorf("url = %q, want regular URL", url)
	}
}

func TestGetImageURL_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	if _, err := getImageURL("k"); err == nil {
		t.Fatal("expected JSON error")
	}
}
