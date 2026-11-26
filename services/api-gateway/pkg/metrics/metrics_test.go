package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinMiddlewareBoundsLabels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	Init()

	router := gin.New()
	router.Use(GinMiddleware("test"))
	router.GET("/known", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 100; i++ {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, fmt.Sprintf("/missing/%d", i), nil))
	}
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("FOO", "/known", nil))

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var series []string
	for _, mf := range families {
		if mf.GetName() != "microblog_http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			series = append(series, fmt.Sprintf("%s %s %s=%v", labels["method"], labels["route"], labels["status"], m.GetCounter().GetValue()))
		}
	}

	want := map[string]bool{
		"GET unmatched 404=100": true,
		"other unmatched 404=1": true,
	}
	if len(series) != len(want) {
		t.Fatalf("got series %v, want %v", series, want)
	}
	for _, s := range series {
		if !want[s] {
			t.Fatalf("unexpected series %q in %v", s, series)
		}
	}
}
