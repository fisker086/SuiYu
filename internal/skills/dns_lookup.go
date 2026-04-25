package skills

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolDNSLookup = "builtin_dns_lookup"

var allowedDNSTypes = map[string]bool{
	"A":     true,
	"AAAA":  true,
	"MX":    true,
	"TXT":   true,
	"NS":    true,
	"CNAME": true,
	"SOA":   true,
	"SRV":   true,
	"PTR":   true,
}

func execBuiltinDNSLookup(_ context.Context, in map[string]any) (string, error) {
	domain := strArg(in, "domain", "host", "name", "target")
	if domain == "" {
		return "", fmt.Errorf("missing domain")
	}

	recordType := strArg(in, "record_type", "type", "rtype")
	if recordType == "" {
		recordType = "A"
	}
	recordType = strings.ToUpper(recordType)

	if !allowedDNSTypes[recordType] {
		return "", fmt.Errorf("record type %q not supported (allowed: %v)", recordType, allowedDNSTypes)
	}

	dnsServer := strArg(in, "dns_server", "server", "resolver")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("DNS %s records for %s:\n\n", recordType, domain))

	switch recordType {
	case "A":
		ips, err := net.LookupIP(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				b.WriteString(fmt.Sprintf("  A: %s\n", ip.String()))
			}
		}
	case "AAAA":
		ips, err := net.LookupIP(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		for _, ip := range ips {
			if ip.To4() == nil {
				b.WriteString(fmt.Sprintf("  AAAA: %s\n", ip.String()))
			}
		}
	case "MX":
		mxs, err := net.LookupMX(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		for _, mx := range mxs {
			b.WriteString(fmt.Sprintf("  MX: %s (pref: %d)\n", mx.Host, mx.Pref))
		}
	case "TXT":
		txts, err := net.LookupTXT(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		for _, txt := range txts {
			b.WriteString(fmt.Sprintf("  TXT: %s\n", txt))
		}
	case "NS":
		nss, err := net.LookupNS(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		for _, ns := range nss {
			b.WriteString(fmt.Sprintf("  NS: %s\n", ns.Host))
		}
	case "CNAME":
		cname, err := net.LookupCNAME(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		b.WriteString(fmt.Sprintf("  CNAME: %s\n", cname))
	case "SOA":
		soas, err := net.LookupNS(domain)
		if err != nil {
			return fmt.Sprintf("DNS lookup failed for %s: %v", domain, err), nil
		}
		if len(soas) > 0 {
			b.WriteString(fmt.Sprintf("  NS: %s\n", soas[0].Host))
		}
	default:
		return "", fmt.Errorf("record type %q not implemented yet", recordType)
	}

	if dnsServer != "" {
		b.WriteString(fmt.Sprintf("\nQueried via custom DNS server: %s\n", dnsServer))
	}

	return b.String(), nil
}

func NewBuiltinDNSLookupTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolDNSLookup,
			Desc:  "DNS lookup: query A, AAAA, MX, TXT, NS, CNAME, SOA, SRV, PTR records.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"domain":      {Type: einoschema.String, Desc: "Domain to query (e.g., example.com)", Required: true},
				"record_type": {Type: einoschema.String, Desc: "Record type: A, AAAA, MX, TXT, NS, CNAME, SOA, SRV, PTR", Required: false},
				"dns_server":  {Type: einoschema.String, Desc: "Custom DNS server (optional)", Required: false},
			}),
		},
		execBuiltinDNSLookup,
	)
}
