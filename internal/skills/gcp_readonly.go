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

const toolGCPReadonly = "builtin_gcp_readonly"

var allowedGCPOps = map[string]bool{
	"gce":     true,
	"gke":     true,
	"buckets": true,
	"regions": true,
}

func execBuiltinGCPReadonly(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "gce"
	}
	if !allowedGCPOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only; allowed: %v)", op, allowedGCPOps)
	}

	zone := strArg(in, "zone", "gcp_zone")
	project := strArg(in, "project", "project_id", "gcp_project")

	var cmd *exec.Cmd
	switch op {
	case "gce":
		args := []string{"compute", "instances", "list", "--format", "table(name,zone,status,machineType)"}
		if project != "" {
			args = append([]string{"--project", project}, args...)
		}
		if zone != "" {
			args = append(args, "--zones", zone)
		}
		cmd = exec.Command("gcloud", args...)
	case "gke":
		args := []string{"container", "clusters", "list"}
		if project != "" {
			args = append([]string{"--project", project}, args...)
		}
		cmd = exec.Command("gcloud", args...)
	case "buckets":
		args := []string{"storage", "buckets", "list"}
		if project != "" {
			args = append([]string{"--project", project}, args...)
		}
		cmd = exec.Command("gcloud", args...)
	case "regions":
		args := []string{"compute", "regions", "list"}
		if project != "" {
			args = append([]string{"--project", project}, args...)
		}
		cmd = exec.Command("gcloud", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("GCP %s: %v\n%s", op, err, string(output)), nil
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("GCP %s: no results", op), nil
	}
	return fmt.Sprintf("GCP %s:\n\n%s", op, result), nil
}

func NewBuiltinGCPReadonlyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolGCPReadonly,
			Desc:  "Read-only GCP via gcloud: GCE instances, GKE clusters, storage buckets, regions. Requires gcloud CLI.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: gce, gke, buckets, regions", Required: false},
				"project":   {Type: einoschema.String, Desc: "Optional GCP project id", Required: false},
				"zone":      {Type: einoschema.String, Desc: "Optional zone filter for gce (e.g. us-central1-a)", Required: false},
			}),
		},
		execBuiltinGCPReadonly,
	)
}
