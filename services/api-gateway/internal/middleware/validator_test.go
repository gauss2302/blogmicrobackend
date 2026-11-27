package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

// Runs through a real server so ContentLength comes from net/http's own
// parsing: no body -> 0, chunked -> -1.
func TestRequestValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var calls atomic.Int32
	echo := func(c *gin.Context) {
		calls.Add(1)
		body, _ := io.ReadAll(c.Request.Body)
		c.Data(http.StatusOK, "application/octet-stream", body)
	}
	router := gin.New()
	router.Use(RequestValidator(32))
	router.POST("/", echo)
	router.GET("/", echo)
	router.DELETE("/", echo)
	srv := httptest.NewServer(router)
	defer srv.Close()

	// An io.Reader of unknown length makes the client send chunked with no Content-Length.
	chunked := func(s string) io.Reader { return io.MultiReader(strings.NewReader(s)) }
	big := `{"name":"` + strings.Repeat("x", 64) + `"}`

	tests := []struct {
		name, method, contentType string
		body                      io.Reader
		want                      int
		wantBody                  string // checked when the handler is expected to run
	}{
		{"text/plain body", http.MethodPost, "text/plain", strings.NewReader("hello"), http.StatusUnsupportedMediaType, ""},
		{"form body", http.MethodPost, "application/x-www-form-urlencoded", strings.NewReader("a=b"), http.StatusUnsupportedMediaType, ""},
		{"body without content type", http.MethodPost, "", strings.NewReader(`{"a":1}`), http.StatusUnsupportedMediaType, ""},
		{"chunked without content type", http.MethodPost, "", chunked(`{"a":1}`), http.StatusUnsupportedMediaType, ""},
		{"json-like media type", http.MethodPost, "application/jsonx", strings.NewReader(`{}`), http.StatusUnsupportedMediaType, ""},
		{"oversized json", http.MethodPost, "application/json", strings.NewReader(big), http.StatusRequestEntityTooLarge, ""},
		{"oversized chunked json", http.MethodPost, "application/json", chunked(big), http.StatusRequestEntityTooLarge, ""},
		{"invalid json", http.MethodPost, "application/json", strings.NewReader(`{`), http.StatusBadRequest, ""},
		{"json with charset", http.MethodPost, "application/json; charset=utf-8", strings.NewReader(`{"a": 1}`), http.StatusOK, `{"a": 1}`},
		{"chunked json", http.MethodPost, "application/json", chunked(`{"a":1}`), http.StatusOK, `{"a":1}`},
		{"post without body or content type", http.MethodPost, "", nil, http.StatusOK, ""},
		{"get with text body", http.MethodGet, "text/plain", strings.NewReader("hi"), http.StatusOK, "hi"},
		{"delete with text body", http.MethodDelete, "text/plain", strings.NewReader("hi"), http.StatusOK, "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls.Store(0)
			req, err := http.NewRequest(tt.method, srv.URL, tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", resp.StatusCode, tt.want, got)
			}
			handlerRan := calls.Load() == 1
			if wantRan := tt.want == http.StatusOK; handlerRan != wantRan {
				t.Fatalf("handler ran = %v, want %v", handlerRan, wantRan)
			}
			if handlerRan && string(got) != tt.wantBody {
				t.Fatalf("handler saw body %q, want %q", got, tt.wantBody)
			}
		})
	}
}
