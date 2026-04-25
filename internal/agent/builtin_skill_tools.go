package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/dop251/goja"
	"github.com/expr-lang/expr"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/skills"
)

const (
	builtinSkillSearch           = "builtin_skill.search"
	builtinSkillCalculator       = "builtin_skill.calculator"
	builtinSkillCodeInterpreter  = "builtin_skill.code_interpreter"
	builtinSkillLogAnalyzer      = "builtin_skill.log_analyzer"
	builtinSkillHTTPClient       = "builtin_skill.http_client"
	builtinSkillFileParser       = "builtin_skill.office_doc"
	builtinSkillGitOperator      = "builtin_skill.git_operator"
	builtinSkillSSHExecutor      = "builtin_skill.ssh_executor"
	builtinSkillK8sOperator      = "builtin_skill.k8s_operator"
	builtinSkillDockerOperator   = "builtin_skill.docker_operator"
	builtinSkillDBQuery          = "builtin_skill.db_query"
	builtinSkillAlertSender      = "builtin_skill.alert_sender"
	builtinSkillSystemMonitor    = "builtin_skill.system_monitor"
	builtinSkillCronManager      = "builtin_skill.cron_manager"
	builtinSkillNetworkTools     = "builtin_skill.network_tools"
	builtinSkillCertChecker      = "builtin_skill.cert_checker"
	builtinSkillNginxDiagnose    = "builtin_skill.nginx_diagnose"
	builtinSkillPrometheusQuery  = "builtin_skill.prometheus_query"
	builtinSkillCSVAnalyzer      = "builtin_skill.office_doc"
	builtinSkillGrafanaReader    = "builtin_skill.grafana_reader"
	builtinSkillDNSLookup        = "builtin_skill.dns_lookup"
	builtinSkillAWSReadonly      = "builtin_skill.aws_readonly"
	builtinSkillTerraformPlan    = "builtin_skill.terraform_plan"
	builtinSkillJiraConnector    = "builtin_skill.jira_connector"
	builtinSkillGitHubIssue      = "builtin_skill.github_issue"
	builtinSkillSlackNotify      = "builtin_skill.slack_notify"
	builtinSkillImageAnalyzer    = "builtin_skill.image_analyzer"
	builtinSkillBrowserClient    = "builtin_skill.browser_client"
	builtinSkillVisibleBrowser   = "builtin_skill.visible_browser"
	builtinSkillHTTPTest         = "builtin_skill.http_test"
	builtinSkillTestRunner       = "builtin_skill.test_runner"
	builtinSkillTestReportParser = "builtin_skill.test_report_parser"
	builtinSkillLoadTest         = "builtin_skill.load_test"
	builtinSkillRedisTool        = "builtin_skill.redis_tool"
	builtinSkillMySQLExplain     = "builtin_skill.mysql_explain"
	builtinSkillESQuery          = "builtin_skill.es_query"
	builtinSkillS3Tool           = "builtin_skill.s3_tool"
	builtinSkillHuaweiSwitch     = "builtin_skill.huawei_switch"
	builtinSkillH3CSwitch        = "builtin_skill.h3c_switch"
	builtinSkillCiscoIOS         = "builtin_skill.cisco_ios"
	builtinSkillGCPReadonly      = "builtin_skill.gcp_readonly"
	builtinSkillAzureReadonly    = "builtin_skill.azure_readonly"
	builtinSkillLokiQuery        = "builtin_skill.loki_query"
	builtinSkillArgoCDReadonly   = "builtin_skill.argocd_readonly"

	toolWebSearch     = "builtin_web_search"
	toolHTTPClient    = "builtin_http_client"
	toolCalculate     = "builtin_calculate"
	toolRunJavaScript = "builtin_run_javascript"

	maxFetchBodyBytes = 2 << 20 // 2 MiB
	fetchTimeout      = 25 * time.Second
	codeRunTimeout    = 8 * time.Second
	maxJSCodeChars    = 32000
)

// dedupeBaseToolsByName keeps the first tool for each Info().Name. API providers (e.g. Vertex) reject
// duplicate function declarations in one request.
func dedupeBaseToolsByName(in []tool.BaseTool) []tool.BaseTool {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]tool.BaseTool, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		info, err := t.Info(context.Background())
		if err != nil || info == nil || strings.TrimSpace(info.Name) == "" {
			out = append(out, t)
			continue
		}
		name := info.Name
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, t)
	}
	return out
}

// skillIDsForAgent returns bound skill keys (e.g. builtin_skill.search) from runtime profile or agent.
func skillIDsForAgent(agent *schema.AgentWithRuntime) []string {
	if agent == nil {
		return nil
	}
	if agent.RuntimeProfile != nil && len(agent.RuntimeProfile.SkillIDs) > 0 {
		logger.Debug("skill IDs from runtime profile", "agent_id", agent.ID, "skill_ids", agent.RuntimeProfile.SkillIDs)
		return agent.RuntimeProfile.SkillIDs
	}
	logger.Debug("skill IDs from agent", "agent_id", agent.ID, "skill_ids", agent.SkillIDs)
	return agent.SkillIDs
}

// allToolsForAgent merges MCP tools and builtin skill tools for the chat model.
func (r *Runtime) allToolsForAgent(agent *schema.AgentWithRuntime) ([]tool.BaseTool, error) {
	mcpTools, err := r.mcpToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	builtin, err := r.builtinSkillToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	if len(builtin) == 0 {
		return dedupeBaseToolsByName(mcpTools), nil
	}
	out := make([]tool.BaseTool, 0, len(mcpTools)+len(builtin))
	out = append(out, mcpTools...)
	out = append(out, builtin...)
	return dedupeBaseToolsByName(out), nil
}

// AllToolsForAgent exposes the same merged tool list as the chat runtime (e.g. SubmitToolResult resume).
func (r *Runtime) AllToolsForAgent(agent *schema.AgentWithRuntime) ([]tool.BaseTool, error) {
	return r.allToolsForAgent(agent)
}

func (r *Runtime) builtinSkillToolsForAgent(agent *schema.AgentWithRuntime) ([]tool.BaseTool, error) {
	ids := skillIDsForAgent(agent)
	if len(ids) == 0 {
		return nil, nil
	}
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[strings.TrimSpace(id)] = struct{}{}
	}

	var out []tool.BaseTool
	var errs []string
	agentID := int64(0)
	if agent != nil {
		agentID = agent.ID
	}

	safeAppendTool := func(t tool.BaseTool, skillKey string) {
		if t == nil {
			errs = append(errs, fmt.Sprintf("skill %q returned nil tool", skillKey))
			return
		}
		out = append(out, t)
	}

	if _, ok := want[builtinSkillSearch]; ok {
		safeAppendTool(skills.NewBuiltinHTTPClientTool(), builtinSkillSearch)
		safeAppendTool(newBuiltinWebSearchTool(), builtinSkillSearch+"_search")
	}
	if _, ok := want[builtinSkillCalculator]; ok {
		safeAppendTool(newBuiltinCalculateTool(), builtinSkillCalculator)
	}
	if _, ok := want[builtinSkillCodeInterpreter]; ok {
		safeAppendTool(newBuiltinRunJavaScriptTool(), builtinSkillCodeInterpreter)
	}
	if _, ok := want[builtinSkillLogAnalyzer]; ok {
		safeAppendTool(skills.NewBuiltinLogAnalyzerTool(), builtinSkillLogAnalyzer)
	}
	if _, ok := want[builtinSkillHTTPClient]; ok {
		safeAppendTool(skills.NewBuiltinHTTPClientTool(), builtinSkillHTTPClient)
	}
	// file_parser and csv_analyzer share skill id builtin_skill.office_doc (one tool: builtin_office_doc).
	if _, ok := want[builtinSkillFileParser]; ok {
		safeAppendTool(skills.NewBuiltinOfficeDocTool(), builtinSkillFileParser)
	}
	if _, ok := want[builtinSkillGitOperator]; ok {
		safeAppendTool(skills.NewBuiltinGitOperatorTool(), builtinSkillGitOperator)
	}
	if _, ok := want[builtinSkillSSHExecutor]; ok {
		safeAppendTool(skills.NewBuiltinSSHExecutorTool(), builtinSkillSSHExecutor)
	}
	if _, ok := want[builtinSkillK8sOperator]; ok {
		safeAppendTool(skills.NewBuiltinK8sOperatorTool(), builtinSkillK8sOperator)
	}
	if _, ok := want[builtinSkillDockerOperator]; ok {
		safeAppendTool(skills.NewBuiltinDockerOperatorTool(), builtinSkillDockerOperator)
	}
	if _, ok := want[builtinSkillDBQuery]; ok {
		safeAppendTool(skills.NewBuiltinDBQueryTool(), builtinSkillDBQuery)
	}
	if _, ok := want[builtinSkillAlertSender]; ok {
		safeAppendTool(skills.NewBuiltinAlertSenderTool(), builtinSkillAlertSender)
	}
	if _, ok := want[builtinSkillSystemMonitor]; ok {
		safeAppendTool(skills.NewBuiltinSystemMonitorTool(), builtinSkillSystemMonitor)
	}
	if _, ok := want[builtinSkillCronManager]; ok {
		safeAppendTool(skills.NewBuiltinCronManagerTool(), builtinSkillCronManager)
	}
	if _, ok := want[builtinSkillNetworkTools]; ok {
		safeAppendTool(skills.NewBuiltinNetworkToolsTool(), builtinSkillNetworkTools)
	}
	if _, ok := want[builtinSkillCertChecker]; ok {
		safeAppendTool(skills.NewBuiltinCertCheckerTool(), builtinSkillCertChecker)
	}
	if _, ok := want[builtinSkillNginxDiagnose]; ok {
		safeAppendTool(skills.NewBuiltinNginxDiagnoseTool(), builtinSkillNginxDiagnose)
	}
	if _, ok := want[builtinSkillPrometheusQuery]; ok {
		safeAppendTool(skills.NewBuiltinPrometheusQueryTool(), builtinSkillPrometheusQuery)
	}
	if _, ok := want[builtinSkillGrafanaReader]; ok {
		safeAppendTool(skills.NewBuiltinGrafanaReaderTool(), builtinSkillGrafanaReader)
	}
	if _, ok := want[builtinSkillDNSLookup]; ok {
		safeAppendTool(skills.NewBuiltinDNSLookupTool(), builtinSkillDNSLookup)
	}
	if _, ok := want[builtinSkillAWSReadonly]; ok {
		safeAppendTool(skills.NewBuiltinAWSReadonlyTool(), builtinSkillAWSReadonly)
	}
	if _, ok := want[builtinSkillTerraformPlan]; ok {
		safeAppendTool(skills.NewBuiltinTerraformPlanTool(), builtinSkillTerraformPlan)
	}
	if _, ok := want[builtinSkillJiraConnector]; ok {
		safeAppendTool(skills.NewBuiltinJiraConnectorTool(), builtinSkillJiraConnector)
	}
	if _, ok := want[builtinSkillGitHubIssue]; ok {
		safeAppendTool(skills.NewBuiltinGitHubIssueTool(), builtinSkillGitHubIssue)
	}
	if _, ok := want[builtinSkillSlackNotify]; ok {
		safeAppendTool(skills.NewBuiltinSlackNotifyTool(), builtinSkillSlackNotify)
	}
	if _, ok := want[builtinSkillImageAnalyzer]; ok {
		safeAppendTool(skills.NewBuiltinImageAnalyzerTool(), builtinSkillImageAnalyzer)
	}
	if _, ok := want[builtinSkillBrowserClient]; ok {
		safeAppendTool(skills.NewBuiltinBrowserTool(), builtinSkillBrowserClient)
	}
	if _, ok := want[builtinSkillVisibleBrowser]; ok {
		safeAppendTool(skills.NewBuiltinVisibleBrowserTool(), builtinSkillVisibleBrowser)
	}
	if _, ok := want[builtinSkillHTTPTest]; ok {
		safeAppendTool(skills.NewBuiltinHTTPTestTool(), builtinSkillHTTPTest)
	}
	if _, ok := want[builtinSkillTestRunner]; ok {
		safeAppendTool(skills.NewBuiltinTestRunnerTool(), builtinSkillTestRunner)
	}
	if _, ok := want[builtinSkillTestReportParser]; ok {
		safeAppendTool(skills.NewBuiltinTestReportParserTool(), builtinSkillTestReportParser)
	}
	if _, ok := want[builtinSkillLoadTest]; ok {
		safeAppendTool(skills.NewBuiltinLoadTestTool(), builtinSkillLoadTest)
	}
	if _, ok := want[builtinSkillRedisTool]; ok {
		safeAppendTool(skills.NewBuiltinRedisTool(), builtinSkillRedisTool)
	}
	if _, ok := want[builtinSkillMySQLExplain]; ok {
		safeAppendTool(skills.NewBuiltinMySQLExplainTool(), builtinSkillMySQLExplain)
	}
	if _, ok := want[builtinSkillESQuery]; ok {
		safeAppendTool(skills.NewBuiltinESQueryTool(), builtinSkillESQuery)
	}
	if _, ok := want[builtinSkillS3Tool]; ok {
		safeAppendTool(skills.NewBuiltinS3Tool(), builtinSkillS3Tool)
	}
	if _, ok := want[builtinSkillHuaweiSwitch]; ok {
		safeAppendTool(skills.NewBuiltinHuaweiSwitchTool(), builtinSkillHuaweiSwitch)
	}
	if _, ok := want[builtinSkillH3CSwitch]; ok {
		safeAppendTool(skills.NewBuiltinH3CSwitchTool(), builtinSkillH3CSwitch)
	}
	if _, ok := want[builtinSkillCiscoIOS]; ok {
		safeAppendTool(skills.NewBuiltinCiscoIOSTool(), builtinSkillCiscoIOS)
	}
	if _, ok := want[builtinSkillGCPReadonly]; ok {
		safeAppendTool(skills.NewBuiltinGCPReadonlyTool(), builtinSkillGCPReadonly)
	}
	if _, ok := want[builtinSkillAzureReadonly]; ok {
		safeAppendTool(skills.NewBuiltinAzureReadonlyTool(), builtinSkillAzureReadonly)
	}
	if _, ok := want[builtinSkillLokiQuery]; ok {
		safeAppendTool(skills.NewBuiltinLokiQueryTool(), builtinSkillLokiQuery)
	}
	if _, ok := want[builtinSkillArgoCDReadonly]; ok {
		safeAppendTool(skills.NewBuiltinArgoCDReadonlyTool(), builtinSkillArgoCDReadonly)
	}
	if len(errs) > 0 {
		logger.Error("failed to load builtin skill tools for agent", "agent_id", agentID, "errors", errs)
		return nil, fmt.Errorf("failed to load builtin skill tools: %s", strings.Join(errs, "; "))
	}
	if len(out) > 0 {
		toolNames := make([]string, len(out))
		for i, t := range out {
			info, _ := t.Info(context.Background())
			toolNames[i] = info.Name
		}
		logger.Info("builtin skill tools bound for agent", "agent_id", agentID, "skill_ids", ids, "tool_count", len(out), "tool_names", toolNames)
	}
	return out, nil
}

// skillUsageHintsFromAgent adds instruction text so the model uses builtin tools instead of refusing.
func (r *Runtime) skillUsageHintsFromAgent(agent *schema.AgentWithRuntime) string {
	ids := skillIDsForAgent(agent)
	if len(ids) == 0 {
		return ""
	}
	want := make(map[string]struct{})
	for _, id := range ids {
		want[strings.TrimSpace(id)] = struct{}{}
	}
	var parts []string
	if _, ok := want[builtinSkillSearch]; ok {
		parts = append(parts, "- **Web / links**: Use `"+toolHTTPClient+"` with an **https** URL to read page text (HTML is automatically stripped). Use `"+toolWebSearch+"` with a **query** for quick facts (DuckDuckGo). Do not claim you cannot open links if these tools are available.")
	}
	if _, ok := want[builtinSkillCalculator]; ok {
		parts = append(parts, "- **Math**: Use `"+toolCalculate+"` with field `expression` (safe arithmetic / comparisons).")
	}
	if _, ok := want[builtinSkillCodeInterpreter]; ok {
		parts = append(parts, "- **Code**: Use `"+toolRunJavaScript+"` with field `code` for short JavaScript snippets (sandboxed, timeout). `console.log` / `info` / `warn` / `error` capture output; no Node/fs/network.")
	}
	if _, ok := want[builtinSkillLogAnalyzer]; ok {
		parts = append(parts, "- **Logs**: Use `builtin_log_analyzer` to parse, filter, or summarize logs (nginx, syslog, JSON formats). (Runs on client)")
	}
	if _, ok := want[builtinSkillHTTPClient]; ok {
		parts = append(parts, "- **HTTP**: Use `builtin_http_client` for full HTTP requests (GET/POST/PUT/DELETE) with custom headers and body (https only).")
	}
	if _, ok := want[builtinSkillFileParser]; ok {
		parts = append(parts, "- **Files**: Use `builtin_office_doc` to parse PDF, Excel, CSV, Word, YAML, JSON, INI, TOML, XML. (Runs on client)")
	}
	if _, ok := want[builtinSkillGitOperator]; ok {
		parts = append(parts, "- **Git**: Use `builtin_git_operator` for read-only git operations (status, log, diff, branch, blame). (Runs on client)")
	}
	if _, ok := want[builtinSkillSSHExecutor]; ok {
		parts = append(parts, "- **SSH**: Use `builtin_ssh_executor` for read-only remote server commands (check status, logs, processes).")
	}
	if _, ok := want[builtinSkillK8sOperator]; ok {
		parts = append(parts, "- **K8s**: Use `builtin_k8s_operator` for read-only Kubernetes operations (get pods, describe, logs, events).")
	}
	if _, ok := want[builtinSkillDockerOperator]; ok {
		parts = append(parts, "- **Docker**: Use `builtin_docker_operator` for read-only Docker operations (ps, logs, inspect, stats). (Runs on client)")
	}
	if _, ok := want[builtinSkillDBQuery]; ok {
		parts = append(parts, "- **Database**: Use `builtin_db_query` for read-only SQL queries (SELECT only) on MySQL/PostgreSQL.")
	}
	if _, ok := want[builtinSkillAlertSender]; ok {
		parts = append(parts, "- **Alerts**: Use `builtin_alert_sender` to send notifications to Lark, DingTalk, or WeCom via webhook.")
	}
	if _, ok := want[builtinSkillSystemMonitor]; ok {
		parts = append(parts, "- **System**: Use `builtin_system_monitor` to check CPU, memory, disk, processes, uptime. (Runs on client)")
	}
	if _, ok := want[builtinSkillCronManager]; ok {
		parts = append(parts, "- **Cron**: Use `builtin_cron_manager` to list/read/write user crontab (write/append/clear) or read system/status. (Runs on client; writes need user confirmation.)")
	}
	if _, ok := want[builtinSkillNetworkTools]; ok {
		parts = append(parts, "- **Network**: Use `builtin_network_tools` for ping, traceroute, connections, listening ports. (Runs on client)")
	}
	if _, ok := want[builtinSkillCertChecker]; ok {
		parts = append(parts, "- **Cert**: Use `builtin_cert_checker` to check SSL/TLS certificate expiry for domains. (Runs on client)")
	}
	if _, ok := want[builtinSkillNginxDiagnose]; ok {
		parts = append(parts, "- **Nginx**: Use `builtin_nginx_diagnose` to test config, show config, list sites, check status. (Runs on client)")
	}
	if _, ok := want[builtinSkillPrometheusQuery]; ok {
		parts = append(parts, "- **Prometheus**: Use `builtin_prometheus_query` to query metrics, alerts, targets via PromQL.")
	}
	if _, ok := want[builtinSkillCSVAnalyzer]; ok {
		parts = append(parts, "- **CSV**: Use `builtin_office_doc` format=csv to analyze spreadsheet data. (Runs on client)")
	}
	if _, ok := want[builtinSkillGrafanaReader]; ok {
		parts = append(parts, "- **Grafana**: Use `builtin_grafana_reader` to read dashboards, panels, alerts, datasources.")
	}
	if _, ok := want[builtinSkillDNSLookup]; ok {
		parts = append(parts, "- **DNS**: Use `builtin_dns_lookup` to query A, AAAA, MX, TXT, NS, CNAME, SOA records. (Runs on client)")
	}
	if _, ok := want[builtinSkillAWSReadonly]; ok {
		parts = append(parts, "- **AWS**: Use `builtin_aws_readonly` for read-only AWS operations (EC2, S3, RDS, CloudWatch).")
	}
	if _, ok := want[builtinSkillGCPReadonly]; ok {
		parts = append(parts, "- **GCP**: Use `builtin_gcp_readonly` for read-only gcloud (GCE, GKE, buckets, regions).")
	}
	if _, ok := want[builtinSkillAzureReadonly]; ok {
		parts = append(parts, "- **Azure**: Use `builtin_azure_readonly` for read-only az (VMs, groups, storage, AKS).")
	}
	if _, ok := want[builtinSkillLokiQuery]; ok {
		parts = append(parts, "- **Loki**: Use `builtin_loki_query` for LogQL and labels against a Loki base URL.")
	}
	if _, ok := want[builtinSkillArgoCDReadonly]; ok {
		parts = append(parts, "- **Argo CD**: Use `builtin_argocd_readonly` for kubectl read-only on Applications / AppProjects (argoproj.io).")
	}
	if _, ok := want[builtinSkillTerraformPlan]; ok {
		parts = append(parts, "- **Terraform**: Use `builtin_terraform_plan` to preview changes, list state, validate config.")
	}
	if _, ok := want[builtinSkillJiraConnector]; ok {
		parts = append(parts, "- **Jira**: Use `builtin_jira_connector` to search issues, get details, list projects.")
	}
	if _, ok := want[builtinSkillGitHubIssue]; ok {
		parts = append(parts, "- **GitHub**: Use `builtin_github_issue` to list issues, PRs, repo info.")
	}
	if _, ok := want[builtinSkillSlackNotify]; ok {
		parts = append(parts, "- **Slack**: Use `builtin_slack_notify` to send messages via webhook or API.")
	}
	if _, ok := want[builtinSkillImageAnalyzer]; ok {
		parts = append(parts, "- **Image**: Use `builtin_image_analyzer` for technical image details (dimensions, format, size). For visual content, use model vision.")
	}
	// Browser: one line whether the agent binds browser_client and/or legacy visible_browser (stub tool).
	_, hasBrowserClient := want[builtinSkillBrowserClient]
	_, hasVisibleBrowser := want[builtinSkillVisibleBrowser]
	if hasBrowserClient || hasVisibleBrowser {
		parts = append(parts, "- **Browser**: `builtin_browser` runs **only on the desktop client**: opens a **visible** Chrome/Chromium window (CDP: navigate, click, type, screenshot, page text for the model). It is **not** headless and **not** the same profile as the user’s everyday Chrome. Use `builtin_http_client` for **server-side** HTTP only.")
	}
	if len(parts) == 0 {
		return ""
	}
	return "## Builtin skills (bound to this agent)\n\n" + strings.Join(parts, "\n")
}

func strArg(in map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := in[k]
		if !ok || v == nil {
			continue
		}
		switch s := v.(type) {
		case string:
			return s
		default:
			return fmt.Sprint(s)
		}
	}
	return ""
}

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

func hostLooksUnsafe(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "localhost" {
		return true
	}
	if strings.HasPrefix(host, "[") {
		return true
	}
	h := host
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") {
		// host:port
		h = host[:i]
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
	}
	return false
}

func execBuiltinWebSearch(_ context.Context, in map[string]any) (string, error) {
	q := strings.TrimSpace(strArg(in, "query", "q", "search"))
	if q == "" {
		return "", fmt.Errorf("missing query")
	}
	u := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(q)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "sya-agent-builtin-search/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var ddg struct {
		Abstract       string `json:"Abstract"`
		AbstractURL    string `json:"AbstractURL"`
		Answer         string `json:"Answer"`
		Definition     string `json:"Definition"`
		RelatedTopics  []any  `json:"RelatedTopics"`
		Results        []any  `json:"Results"`
		Infobox        any    `json:"Infobox"`
		Heading        string `json:"Heading"`
		AbstractSource string `json:"AbstractSource"`
	}
	if err := json.Unmarshal(body, &ddg); err != nil {
		return "", fmt.Errorf("parse search response: %w", err)
	}
	var b strings.Builder
	if ddg.Answer != "" {
		b.WriteString("Answer: ")
		b.WriteString(ddg.Answer)
		b.WriteString("\n")
	}
	if ddg.Abstract != "" {
		b.WriteString("Abstract: ")
		b.WriteString(ddg.Abstract)
		if ddg.AbstractURL != "" {
			b.WriteString(" (")
			b.WriteString(ddg.AbstractURL)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	if ddg.Definition != "" {
		b.WriteString("Definition: ")
		b.WriteString(ddg.Definition)
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "No instant answer from search API. Try builtin_fetch_url with a specific https link, or rephrase the query.", nil
	}
	return strings.TrimSpace(b.String()), nil
}

func execBuiltinCalculate(_ context.Context, in map[string]any) (string, error) {
	e := strings.TrimSpace(strArg(in, "expression", "expr", "formula"))
	if e == "" {
		return "", fmt.Errorf("missing expression")
	}
	out, err := expr.Eval(e, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(out), nil
}

func execBuiltinRunJavaScript(_ context.Context, in map[string]any) (string, error) {
	src := strArg(in, "code", "script", "javascript")
	if len(src) > maxJSCodeChars {
		return "", fmt.Errorf("code too long (max %d chars)", maxJSCodeChars)
	}
	vm := goja.New()
	var logBuf strings.Builder
	printLine := func(call goja.FunctionCall) goja.Value {
		for i, arg := range call.Arguments {
			if i > 0 {
				logBuf.WriteString(" ")
			}
			logBuf.WriteString(arg.String())
		}
		logBuf.WriteByte('\n')
		return goja.Undefined()
	}
	console := vm.NewObject()
	_ = console.Set("log", printLine)
	_ = console.Set("info", printLine)
	_ = console.Set("warn", printLine)
	_ = console.Set("error", printLine)
	_ = console.Set("debug", printLine)
	_ = vm.Set("console", console)

	timer := time.AfterFunc(codeRunTimeout, func() {
		vm.Interrupt("timeout")
	})
	defer timer.Stop()
	v, err := vm.RunString(src)
	if err != nil {
		return "", err
	}
	logOut := strings.TrimSpace(logBuf.String())
	var retOut string
	if v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		retOut = strings.TrimSpace(v.String())
	}
	switch {
	case logOut != "" && retOut != "":
		return logOut + "\n\n" + retOut, nil
	case logOut != "":
		return logOut, nil
	case retOut == "":
		return "(no value)", nil
	default:
		return retOut, nil
	}
}

func newBuiltinWebSearchTool() tool.BaseTool {
	ti := &einoschema.ToolInfo{
		Name: toolWebSearch,
		Desc: "Quick web lookup via DuckDuckGo instant answer API (facts, definitions). For full articles, use " + toolHTTPClient + " with an https link.",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"query": {Type: einoschema.String, Desc: "Search query", Required: true},
		}),
	}
	return toolutils.NewTool(ti, execBuiltinWebSearch)
}

func newBuiltinCalculateTool() tool.BaseTool {
	ti := &einoschema.ToolInfo{
		Name: toolCalculate,
		Desc: "Evaluate a safe arithmetic / boolean expression (expr-lang syntax: + - * / % **, comparisons, and functions like min, max).",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"expression": {Type: einoschema.String, Desc: "Expression to evaluate", Required: true},
		}),
	}
	return toolutils.NewTool(ti, execBuiltinCalculate)
}

func newBuiltinRunJavaScriptTool() tool.BaseTool {
	ti := &einoschema.ToolInfo{
		Name: toolRunJavaScript,
		Desc: "Run a short JavaScript snippet in a sandboxed engine (no Node/fs/network). console.log/info/warn/error/debug are supported. Stops after " + codeRunTimeout.String() + ".",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"code": {Type: einoschema.String, Desc: "JavaScript source", Required: true},
		}),
	}
	return toolutils.NewTool(ti, execBuiltinRunJavaScript)
}
