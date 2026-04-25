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

const toolAWSReadonly = "builtin_aws_readonly"

var allowedAWSOps = map[string]bool{
	"ec2":     true,
	"s3":      true,
	"rds":     true,
	"alarms":  true,
	"regions": true,
}

func execBuiltinAWSReadonly(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "ec2"
	}

	if !allowedAWSOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedAWSOps)
	}

	region := strArg(in, "region", "aws_region")
	if region == "" {
		region = "us-east-1"
	}

	var cmd *exec.Cmd
	switch op {
	case "ec2":
		cmd = exec.Command("aws", "ec2", "describe-instances", "--region", region, "--query", "Reservations[].Instances[].{ID:InstanceId,Type:InstanceType,State:State.Name,Name:Tags[?Key=='Name'].Value|[0]}", "--output", "table")
	case "s3":
		cmd = exec.Command("aws", "s3", "ls", "--region", region)
	case "rds":
		cmd = exec.Command("aws", "rds", "describe-db-instances", "--region", region, "--query", "DBInstances[].{ID:DBInstanceIdentifier,Engine:Engine,Status:DBInstanceStatus}", "--output", "table")
	case "alarms":
		cmd = exec.Command("aws", "cloudwatch", "describe-alarms", "--region", region, "--query", "MetricAlarms[].{Name:AlarmName,State:StateValue}", "--output", "table")
	case "regions":
		cmd = exec.Command("aws", "ec2", "describe-regions", "--query", "Regions[].RegionName", "--output", "table")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("AWS %s (region %s): %v\n%s", op, region, err, string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("AWS %s (region %s): no results", op, region), nil
	}

	return fmt.Sprintf("AWS %s (region %s):\n\n%s", op, region, result), nil
}

func NewBuiltinAWSReadonlyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolAWSReadonly,
			Desc:  "Read-only AWS operations: EC2, S3, RDS, CloudWatch alarms. Requires AWS CLI.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: ec2, s3, rds, alarms, regions", Required: false},
				"region":    {Type: einoschema.String, Desc: "AWS region (default: us-east-1)", Required: false},
			}),
		},
		execBuiltinAWSReadonly,
	)
}
