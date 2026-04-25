package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolHTTPClient = "builtin_http_client"

const (
	httpMaxBodyBytes = 2 << 20
	httpTimeout      = 25 * time.Second
)

var htmlTagRe = regexp.MustCompile(`(?i)<script[\s\S]*?</script>|<style[\s\S]*?</style>|<[^>]+>`)

func stripHTMLToText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func execBuiltinHTTPClient(_ context.Context, in map[string]any) (string, error) {
	method := strArg(in, "method", "http_method", "verb")
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	rawURL := strArg(in, "url", "endpoint", "uri")
	if rawURL == "" {
		return "", fmt.Errorf("missing url")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("only https URLs are allowed")
	}
	if hostLooksUnsafe(u.Hostname()) {
		return "", fmt.Errorf("host is not allowed (private or local addresses blocked)")
	}

	reqBody := strArg(in, "body", "payload", "data")
	contentType := strArg(in, "content_type", "contentType", "type")
	if contentType == "" {
		contentType = "application/json"
	}

	var bodyReader io.Reader
	if reqBody != "" {
		bodyReader = strings.NewReader(reqBody)
	}

	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "sya-agent-builtin-http/1.0")

	headersJSON := strArg(in, "headers", "header", "custom_headers")
	if headersJSON != "" {
		var headers map[string]string
		if err := jsonUnmarshal(headersJSON, &headers); err != nil {
			return "", fmt.Errorf("invalid headers JSON: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, httpMaxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	bodyStr := string(respBody)
	trimmed := strings.TrimSpace(bodyStr)
	trimmed = strings.TrimPrefix(trimmed, "\ufeff")
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	isJSON := strings.Contains(ct, "application/json") || strings.Contains(ct, "+json") || strings.HasSuffix(ct, "/json")
	if !isJSON && len(trimmed) > 0 {
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
			isJSON = true
		}
	}

	// 勿把完整响应头喂给模型：会刷屏且与正文无关；调试需要时可后续加 include_headers 参数。
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Status: %s\n", resp.Status))
	if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
		sb.WriteString(fmt.Sprintf("Content-Type: %s\n", ct))
	}

	if isJSON {
		if len(bodyStr) > 120000 {
			bodyStr = bodyStr[:120000] + "\n\n[truncated]"
		}
		if trimmed == "" {
			sb.WriteString("\nBody: (empty or non-text response)")
		} else {
			sb.WriteString(fmt.Sprintf("\nBody:\n%s", bodyStr))
		}
	} else {
		text := stripHTMLToText(bodyStr)
		if len(text) > 120000 {
			text = text[:120000] + "\n\n[truncated]"
		}
		if text == "" {
			sb.WriteString("\nBody: (empty or non-text response)")
		} else {
			sb.WriteString(fmt.Sprintf("\nBody:\n%s", text))
		}
	}

	return sb.String(), nil
}

func NewBuiltinHTTPClientTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolHTTPClient,
			Desc:  "Full-featured HTTP client supporting GET/POST/PUT/PATCH/DELETE with custom headers and body (https only, private hosts blocked).",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"method":       {Type: einoschema.String, Desc: "HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS", Required: false},
				"url":          {Type: einoschema.String, Desc: "Target https URL", Required: true},
				"headers":      {Type: einoschema.String, Desc: "JSON object of custom headers", Required: false},
				"body":         {Type: einoschema.String, Desc: "Request body", Required: false},
				"content_type": {Type: einoschema.String, Desc: "Content-Type for body (default: application/json)", Required: false},
			}),
		},
		execBuiltinHTTPClient,
	)
}
