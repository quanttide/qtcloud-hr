package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteUserInfoAuthorizerRequiresBearer(t *testing.T) {
	authorizer, err := NewRemoteUserInfoAuthorizer("https://auth.example.test/userinfo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorizer.Authorize(context.Background(), ""); err == nil {
		t.Fatal("expected missing bearer token to be rejected")
	}
}

func TestRemoteUserInfoAuthorizerReturnsSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sub":"user-001","nickname":"测试用户"}`))
	}))
	defer server.Close()

	authorizer, err := NewRemoteUserInfoAuthorizer(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	principal, err := authorizer.Authorize(context.Background(), "Bearer token")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "user-001" {
		t.Fatalf("subject = %q", principal.Subject)
	}
}

func TestRemoteUserInfoAuthorizerRejectsNonSuccessAndMissingSubject(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{name: "non success", body: `{"error":"invalid_token"}`, code: http.StatusUnauthorized},
		{name: "missing subject", body: `{}`, code: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			authorizer, err := NewRemoteUserInfoAuthorizer(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := authorizer.Authorize(context.Background(), "Bearer token"); err == nil {
				t.Fatal("expected authorization to fail")
			}
		})
	}
}
