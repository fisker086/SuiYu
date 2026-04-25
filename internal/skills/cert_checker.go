package skills

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolCertChecker = "builtin_cert_checker"

func execBuiltinCertChecker(_ context.Context, in map[string]any) (string, error) {
	domain := strArg(in, "domain", "host", "url", "address")
	if domain == "" {
		return "", fmt.Errorf("missing domain")
	}

	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	domain = strings.Split(domain, "/")[0]

	port := strArg(in, "port", "p")
	if port == "" {
		port = "443"
	}

	addr := domain + ":" + port
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Sprintf("Failed to connect to %s: %v", addr, err), nil
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Sprintf("No certificates found for %s", addr), nil
	}

	cert := state.PeerCertificates[0]
	now := time.Now()
	daysUntilExpiry := cert.NotAfter.Sub(now).Hours() / 24

	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== Certificate Info for %s ===\n\n", domain))
	b.WriteString(fmt.Sprintf("Subject: %s\n", cert.Subject.CommonName))
	b.WriteString(fmt.Sprintf("Issuer: %s\n", cert.Issuer.CommonName))
	b.WriteString(fmt.Sprintf("Valid From: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("Valid Until: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05")))

	if daysUntilExpiry < 0 {
		b.WriteString(fmt.Sprintf("Status: EXPIRED (%.0f days ago)\n", -daysUntilExpiry))
	} else if daysUntilExpiry < 30 {
		b.WriteString(fmt.Sprintf("Status: WARNING - expiring in %.0f days\n", daysUntilExpiry))
	} else {
		b.WriteString(fmt.Sprintf("Status: OK (%.0f days remaining)\n", daysUntilExpiry))
	}

	if len(state.PeerCertificates) > 1 {
		b.WriteString(fmt.Sprintf("\nCertificate Chain (%d certificates):\n", len(state.PeerCertificates)))
		for i, c := range state.PeerCertificates[1:] {
			b.WriteString(fmt.Sprintf("  %d. %s (expires: %s)\n", i+1, c.Subject.CommonName, c.NotAfter.Format("2006-01-02")))
		}
	}

	return b.String(), nil
}

func NewBuiltinCertCheckerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolCertChecker,
			Desc:  "Check SSL/TLS certificate expiry, issuer, and chain for a domain.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"domain": {Type: einoschema.String, Desc: "Domain to check (e.g., example.com)", Required: true},
				"port":   {Type: einoschema.String, Desc: "Port number (default: 443)", Required: false},
			}),
		},
		execBuiltinCertChecker,
	)
}
