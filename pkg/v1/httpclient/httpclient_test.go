package httpclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGet_ReturnsBodyOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestGet_SetsAcceptHeaders(t *testing.T) {
	var gotAccept, gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotLang = r.Header.Get("Accept-Language")
		w.Write([]byte(`x`))
	}))
	defer srv.Close()

	if _, err := Get(srv.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	if gotLang != "en-US" {
		t.Errorf("Accept-Language = %q, want en-US", gotLang)
	}
}

func TestGet_PropagatesNetworkError(t *testing.T) {
	// Start and immediately stop a server: the URL is well-formed but the port
	// no longer accepts connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := Get(url); err == nil {
		t.Fatal("expected network error after server close, got nil")
	}
}

func TestGet_ReturnsBodyEvenForNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server error`))
	}))
	defer srv.Close()

	// Note: the current implementation does not check the status code. This
	// pins down that behavior — callers receive the body unchanged.
	body, err := Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(string(body), "server error") {
		t.Errorf("body = %q, want it to contain server error", body)
	}
}

func TestGet_InvalidURLReturnsError(t *testing.T) {
	if _, err := Get("://not a url"); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
