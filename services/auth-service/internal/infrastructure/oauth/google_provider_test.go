package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchUserInfoStopsWithContext(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := fetchUserInfo(ctx, srv.Client(), srv.URL); err == nil {
		t.Fatal("want an error from a hanging userinfo endpoint")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fetchUserInfo took %s; want it to stop at the context deadline", elapsed)
	}
}

func TestFetchUserInfoErrorOmitsBody(t *testing.T) {
	const secret = "victim@example.com"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"email":"` + secret + `"}`))
	}))
	defer srv.Close()

	_, err := fetchUserInfo(context.Background(), srv.Client(), srv.URL)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v; want an error without the response body", err)
	}
}
