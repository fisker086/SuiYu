package skills

import (
	"context"
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

const toolHTTPTest = "builtin_http_test"

func execBuiltinHTTPTest(_ context.Context, in map[string]any) (string, error) {
	method := strArg(in, "method", "verb", "http_method")
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
		return "", fmt.Errorf("host is not allowed")
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
	req.Header.Set("User-Agent", "sya-http-test/1.0")

	headersJSON := strArg(in, "headers", "header")
	if headersJSON != "" {
		var headers map[string]string
		if err := jsonUnmarshal(headersJSON, &headers); err != nil {
			return "", fmt.Errorf("invalid headers JSON: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Status: %d %s\n", resp.StatusCode, resp.Status))

	assertionsJSON := strArg(in, "assertions", "assert")
	if assertionsJSON != "" {
		sb.WriteString("\nAssertions:\n")
		var assertions []map[string]string
		if err := jsonUnmarshal(assertionsJSON, &assertions); err != nil {
			return "", fmt.Errorf("invalid assertions JSON: %w", err)
		}
		passed, failed := validateAssertions(resp, string(respBody), assertions)
		sb.WriteString(fmt.Sprintf("  Passed: %d\n", passed))
		sb.WriteString(fmt.Sprintf("  Failed: %d\n", failed))
	} else {
		sb.WriteString(fmt.Sprintf("\nBody:\n%s", string(respBody)))
	}

	return sb.String(), nil
}

func validateAssertions(resp *http.Response, body string, assertions []map[string]string) (int, int) {
	passed, failed := 0, 0
	for _, a := range assertions {
		aType := a["type"]
		expected := a["expected"]

		switch aType {
		case "status":
			if fmt.Sprint(resp.StatusCode) == expected || fmt.Sprint(resp.StatusCode)+" "+resp.Status == expected {
				passed++
			} else {
				failed++
			}
		case "body":
			if strings.Contains(body, expected) {
				passed++
			} else {
				failed++
			}
		case "header":
			headerName := a["header"]
			if resp.Header.Get(headerName) == expected {
				passed++
			} else {
				failed++
			}
		case "json_path":
			path := a["path"]
			var data any
			jsonUnmarshal(body, &data)
			if getJSONPath(data, path) == expected {
				passed++
			} else {
				failed++
			}
		}
	}
	return passed, failed
}

func getJSONPath(data any, path string) string {
	path = strings.TrimPrefix(path, "$.")
	parts := strings.Split(path, ".")
	var current any = data
	for _, p := range parts {
		if m, ok := current.(map[string]any); ok {
			current = m[p]
		} else {
			return ""
		}
	}
	return fmt.Sprint(current)
}

var httpTestPathRe = regexp.MustCompile(`^\$|^[a-zA-Z_][a-zA-Z0-9_]*$`)

func NewBuiltinHTTPTestTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolHTTPTest,
			Desc: "HTTP API testing with assertions for status, headers, and body validation.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"method":       {Type: einoschema.String, Desc: "HTTP method: GET, POST, PUT, PATCH, DELETE", Required: false},
				"url":          {Type: einoschema.String, Desc: "Target https URL", Required: true},
				"headers":      {Type: einoschema.String, Desc: "JSON object of custom headers", Required: false},
				"body":         {Type: einoschema.String, Desc: "Request body", Required: false},
				"content_type": {Type: einoschema.String, Desc: "Content-Type (default: application/json)", Required: false},
				"assertions":   {Type: einoschema.String, Desc: "JSON array of assertions", Required: false},
			}),
		},
		execBuiltinHTTPTest,
	)
}
