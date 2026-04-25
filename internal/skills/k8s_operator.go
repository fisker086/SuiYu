package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolK8sOperator = "builtin_k8s_operator"

var allowedK8sOps = map[string]bool{
	"get":           true,
	"describe":      true,
	"logs":          true,
	"events":        true,
	"top":           true,
	"explain":       true,
	"api-resources": true,
	"version":       true,
}

var blockedK8sVerbs = []string{
	"create", "delete", "apply", "edit", "patch", "replace",
	"run", "exec", "port-forward", "cp", "auth can-i",
}

func execBuiltinK8sOperator(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action", "verb")
	if op == "" {
		op = "get"
	}

	if !allowedK8sOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedK8sOps)
	}

	resource := strArg(in, "resource", "kind", "type")
	if resource == "" && op != "version" && op != "api-resources" {
		return "", fmt.Errorf("missing resource type (e.g., pod, deployment, service)")
	}

	name := strArg(in, "name", "resource_name")
	namespace := strArg(in, "namespace", "ns", "namespace_name")
	kubeconfig := strArg(in, "kubeconfig", "config", "kube_config")

	cmdArgs := []string{}
	if kubeconfig != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", kubeconfig)
	}

	cmdArgs = append(cmdArgs, op)

	if resource != "" {
		cmdArgs = append(cmdArgs, resource)
	}
	if name != "" {
		cmdArgs = append(cmdArgs, name)
	}
	if namespace != "" {
		cmdArgs = append(cmdArgs, "-n", namespace)
	} else if op == "get" || op == "describe" || op == "logs" {
		cmdArgs = append(cmdArgs, "--all-namespaces")
	}

	if op == "get" && name == "" {
		cmdArgs = append(cmdArgs, "-o", "wide")
	}

	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl %s failed: %s\n%s", op, err.Error(), string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("kubectl %s: (no output)", op), nil
	}

	return fmt.Sprintf("kubectl %s result:\n\n%s", op, result), nil
}

func NewBuiltinK8sOperatorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolK8sOperator,
			Desc: "Read-only Kubernetes operations: get, describe, logs, events, top, explain. Write operations are disabled for safety.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: get, describe, logs, events, top, explain", Required: false},
				"resource":   {Type: einoschema.String, Desc: "K8s resource type (pod, deployment, service, etc.)", Required: false},
				"name":       {Type: einoschema.String, Desc: "Specific resource name", Required: false},
				"namespace":  {Type: einoschema.String, Desc: "Target namespace", Required: false},
				"kubeconfig": {Type: einoschema.String, Desc: "Path to kubeconfig file", Required: false},
			}),
		},
		execBuiltinK8sOperator,
	)
}
