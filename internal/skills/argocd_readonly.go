package skills

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolArgoCDReadonly = "builtin_argocd_readonly"

var allowedArgoOps = map[string]bool{
	"list_apps":     true,
	"get_app":       true,
	"describe_app":  true,
	"list_projects": true,
}

// Kubernetes resource name (subset): DNS subdomain style.
var k8sSubdomainName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func validK8sName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if len(p) > 63 || !k8sSubdomainName.MatchString(p) {
			return false
		}
	}
	return true
}

func execBuiltinArgoCDReadonly(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_apps"
	}
	if !allowedArgoOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only; allowed: %v)", op, allowedArgoOps)
	}

	ns := strArg(in, "namespace", "ns")
	if ns == "" {
		ns = "argocd"
	}
	if !validK8sName(ns) {
		return "", fmt.Errorf("invalid namespace")
	}

	kubeconfig := strArg(in, "kubeconfig", "config", "kube_config")
	name := strArg(in, "name", "app_name", "application")

	args := []string{}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}

	switch op {
	case "list_apps":
		args = append(args, "get", "applications.argoproj.io", "-n", ns, "-o", "wide")
	case "list_projects":
		args = append(args, "get", "appprojects.argoproj.io", "-n", ns)
	case "get_app", "describe_app":
		if name == "" || !validK8sName(name) {
			return "", fmt.Errorf("missing or invalid application name")
		}
		verb := "get"
		if op == "describe_app" {
			verb = "describe"
		}
		out := "-o"
		outFmt := "wide"
		if op == "describe_app" {
			out, outFmt = "", ""
		}
		tmp := append(args, verb, "applications.argoproj.io", name, "-n", ns)
		if out != "" {
			tmp = append(tmp, out, outFmt)
		}
		args = tmp
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Argo CD kubectl: %v\n%s", err, string(output)), nil
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("Argo CD %s (ns=%s): no output", op, ns), nil
	}
	return fmt.Sprintf("Argo CD %s (namespace=%s):\n\n%s", op, ns, result), nil
}

func NewBuiltinArgoCDReadonlyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolArgoCDReadonly,
			Desc:  "Read-only Argo CD via kubectl: list/get/describe Applications and AppProjects (argoproj.io). Requires kubectl and CRDs.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: list_apps, get_app, describe_app, list_projects", Required: false},
				"namespace":  {Type: einoschema.String, Desc: "Argo CD install namespace (default: argocd)", Required: false},
				"name":       {Type: einoschema.String, Desc: "Application name (for get_app / describe_app)", Required: false},
				"kubeconfig": {Type: einoschema.String, Desc: "Optional kubeconfig path", Required: false},
			}),
		},
		execBuiltinArgoCDReadonly,
	)
}
