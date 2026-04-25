package skills

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolLoadTest = "builtin_load_test"

type loadTestStats struct {
	total     int64
	success   int64
	failures  int64
	latencies []time.Duration
	mu        sync.Mutex
}

func execBuiltinLoadTest(_ context.Context, in map[string]any) (string, error) {
	url := strArg(in, "url", "endpoint")
	if url == "" {
		return "", fmt.Errorf("missing url")
	}

	method := strArg(in, "method", "verb")
	if method == "" {
		method = "GET"
	}

	concurrency := int64(10)
	if c := strArg(in, "concurrency", "users", "threads"); c != "" {
		fmt.Sscanf(c, "%d", &concurrency)
	}

	requests := int64(100)
	if r := strArg(in, "requests", "count", "num"); r != "" {
		fmt.Sscanf(r, "%d", &requests)
	}

	durationSec := int64(10)
	if d := strArg(in, "duration", "time"); d != "" {
		fmt.Sscanf(d, "%d", &durationSec)
	}

	rate := int64(0)
	if r := strArg(in, "rate", "rps"); r != "" {
		fmt.Sscanf(r, "%d", &rate)
	}

	stats := &loadTestStats{}

	start := time.Now()
	var wg sync.WaitGroup

	rateLimiter := time.NewTicker(time.Second / time.Duration(rate+1))
	defer rateLimiter.Stop()

	for i := int64(0); i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 30 * time.Second}

			for atomic.AddInt64(&stats.total, 1) <= requests {
				if rate > 0 {
					<-rateLimiter.C
				}

				reqStart := time.Now()
				resp, err := http.NewRequest(method, url, nil)
				if err != nil {
					atomic.AddInt64(&stats.failures, 1)
					continue
				}

				r, err := client.Do(resp)
				latency := time.Since(reqStart)

				if err != nil || r.StatusCode >= 400 {
					atomic.AddInt64(&stats.failures, 1)
				} else {
					atomic.AddInt64(&stats.success, 1)
				}

				stats.mu.Lock()
				stats.latencies = append(stats.latencies, latency)
				stats.mu.Unlock()

				if durationSec > 0 && time.Since(start) > time.Duration(durationSec)*time.Second {
					break
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Load Test Results:\n"))
	sb.WriteString(fmt.Sprintf("  Duration: %v\n", elapsed))
	sb.WriteString(fmt.Sprintf("  Total Requests: %d\n", stats.total))
	sb.WriteString(fmt.Sprintf("  Successful: %d\n", stats.success))
	sb.WriteString(fmt.Sprintf("  Failed: %d\n", stats.failures))
	sb.WriteString(fmt.Sprintf("  Requests/sec: %.2f\n", float64(stats.success)/elapsed.Seconds()))

	if len(stats.latencies) > 0 {
		var avg, p50, p95, p99 time.Duration
		var totalLatency int64

		for _, l := range stats.latencies {
			totalLatency += l.Nanoseconds()
		}
		avg = time.Duration(totalLatency / int64(len(stats.latencies)))

		mid := len(stats.latencies) / 2
		p50 = stats.latencies[mid]
		p95 = stats.latencies[int(float64(len(stats.latencies))*0.95)]
		p99 = stats.latencies[int(float64(len(stats.latencies))*0.99)]

		sb.WriteString(fmt.Sprintf("  Latency Avg: %v\n", avg))
		sb.WriteString(fmt.Sprintf("  Latency p50: %v\n", p50))
		sb.WriteString(fmt.Sprintf("  Latency p95: %v\n", p95))
		sb.WriteString(fmt.Sprintf("  Latency p99: %v\n", p99))
	}

	return sb.String(), nil
}

func NewBuiltinLoadTestTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolLoadTest,
			Desc: "Performance and load testing for HTTP APIs with RPS and concurrency control.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"url":         {Type: einoschema.String, Desc: "Target https URL", Required: true},
				"method":      {Type: einoschema.String, Desc: "HTTP method (default: GET)", Required: false},
				"concurrency": {Type: einoschema.String, Desc: "Number of concurrent users", Required: false},
				"requests":    {Type: einoschema.String, Desc: "Total number of requests", Required: false},
				"duration":    {Type: einoschema.String, Desc: "Test duration in seconds", Required: false},
				"rate":        {Type: einoschema.String, Desc: "Target requests per second", Required: false},
			}),
		},
		execBuiltinLoadTest,
	)
}
