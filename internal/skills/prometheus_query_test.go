package skills

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecBuiltinPrometheusQueryEncodesPromQL(t *testing.T) {
	var rawQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		if got := r.URL.Query().Get("query"); got != `topk(5, sort_desc(sum by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))))` {
			t.Fatalf("unexpected query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	out, err := execBuiltinPrometheusQuery(context.Background(), map[string]any{
		"operation":      "query",
		"prometheus_url": server.URL,
		"query":          `topk(5, sort_desc(sum by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))))`,
	})
	if err != nil {
		t.Fatalf("execBuiltinPrometheusQuery returned error: %v", err)
	}
	if !strings.Contains(out, "prometheus query result") {
		t.Fatalf("unexpected output: %q", out)
	}
	if strings.Contains(rawQuery, " ") {
		t.Fatalf("raw query was not encoded: %q", rawQuery)
	}
	if !strings.Contains(rawQuery, "mode%21%3D%22idle%22") {
		t.Fatalf("expected encoded matcher in raw query, got %q", rawQuery)
	}
}

func TestBuildPrometheusURLEncodesSeriesMatcher(t *testing.T) {
	u, err := buildPrometheusURL(
		"https://prometheus.example.com",
		"/api/v1/series",
		map[string]string{"match[]": `{job="prometheus"}`},
	)
	if err != nil {
		t.Fatalf("buildPrometheusURL returned error: %v", err)
	}
	if strings.Contains(u, " ") {
		t.Fatalf("URL contains raw spaces: %q", u)
	}
	if !strings.Contains(u, "match%5B%5D=%7Bjob%3D%22prometheus%22%7D") {
		t.Fatalf("URL did not encode match[] correctly: %q", u)
	}
}
