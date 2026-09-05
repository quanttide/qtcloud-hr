package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecruitmentCacheHomeDefaultsInsideProjectWhenUnset(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("QTCLOUD_HUMAN_CACHE_HOME", "")
	if err := configureRecruitmentCacheHome(); err != nil {
		t.Fatalf("configure cache home: %v", err)
	}

	got := os.Getenv("XDG_CACHE_HOME")
	if !strings.HasSuffix(filepath.ToSlash(got), "/.quanttide/cache") {
		t.Fatalf("XDG_CACHE_HOME = %q, want project .quanttide/cache", got)
	}
}

func TestRecruitmentCacheHomePreservesExplicitOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("QTCLOUD_HUMAN_CACHE_HOME", override)
	if err := configureRecruitmentCacheHome(); err != nil {
		t.Fatalf("configure cache home: %v", err)
	}

	if got := os.Getenv("XDG_CACHE_HOME"); got != override {
		t.Fatalf("XDG_CACHE_HOME = %q, want override %q", got, override)
	}
}

func TestWithCORSAllowsConfiguredLocalFrontend(t *testing.T) {
	handler := withCORS(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		[]string{"http://127.0.0.1:5080"},
	)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/recruitment/inbox/sync", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5080")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5080" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Operator, X-Recruitment-Permission" {
		t.Fatalf("allow headers = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Fatalf("allow methods = %q", got)
	}
}

func TestDefaultCORSOriginsIncludeLocalFlutterPorts(t *testing.T) {
	origins := corsOrigins()
	wants := []string{"http://127.0.0.1:5080", "http://127.0.0.1:5081", "http://127.0.0.1:5082", "http://127.0.0.1:5083"}
	for _, want := range wants {
		found := false
		for _, origin := range origins {
			if origin == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("default CORS origins missing %q: %+v", want, origins)
		}
	}
}

func TestWithCORSRejectsUnconfiguredOrigin(t *testing.T) {
	handler := withCORS(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		[]string{"http://127.0.0.1:5080"},
	)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/recruitment/inbox/sync", nil)
	req.Header.Set("Origin", "https://example.test")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected preflight to end with 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}
