package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolTerraformPlan = "builtin_terraform_plan"

var allowedTFOps = map[string]bool{
	"resources": true,
	"providers": true,
	"modules":   true,
	"validate":  true,
	"summary":   true,
}

var resourceRe = regexp.MustCompile(`(?m)resource\s+"([^"]+)"\s+"([^"]+)"`)
var providerRe = regexp.MustCompile(`(?m)provider\s+"([^"]+)"`)
var moduleRe = regexp.MustCompile(`(?m)module\s+"([^"]+)"`)
var variableRe = regexp.MustCompile(`(?m)variable\s+"([^"]+)"`)
var outputRe = regexp.MustCompile(`(?m)output\s+"([^"]+)"`)
var dataRe = regexp.MustCompile(`(?m)data\s+"([^"]+)"\s+"([^"]+)"`)

func execBuiltinTerraformPlan(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "summary"
	}

	if !allowedTFOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedTFOps)
	}

	path := strArg(in, "path", "dir", "directory")
	if path == "" {
		path = "."
	}

	tfFiles, err := findTFFiles(path)
	if err != nil {
		return "", fmt.Errorf("scan directory: %w", err)
	}

	if len(tfFiles) == 0 {
		return fmt.Sprintf("No .tf files found in %s", path), nil
	}

	var allContent strings.Builder
	for _, f := range tfFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		allContent.WriteString(string(data))
		allContent.WriteString("\n")
	}

	content := allContent.String()

	var b strings.Builder

	switch op {
	case "resources":
		resources := resourceRe.FindAllStringSubmatch(content, -1)
		if len(resources) == 0 {
			return "No resources found in .tf files", nil
		}
		b.WriteString(fmt.Sprintf("Resources (%d):\n\n", len(resources)))
		for _, r := range resources {
			b.WriteString(fmt.Sprintf("  %s.%s\n", r[1], r[2]))
		}

	case "providers":
		providers := providerRe.FindAllStringSubmatch(content, -1)
		if len(providers) == 0 {
			return "No provider blocks found", nil
		}
		b.WriteString(fmt.Sprintf("Providers (%d):\n\n", len(providers)))
		seen := make(map[string]bool)
		for _, p := range providers {
			if !seen[p[1]] {
				b.WriteString(fmt.Sprintf("  %s\n", p[1]))
				seen[p[1]] = true
			}
		}

	case "modules":
		modules := moduleRe.FindAllStringSubmatch(content, -1)
		if len(modules) == 0 {
			return "No module blocks found", nil
		}
		b.WriteString(fmt.Sprintf("Modules (%d):\n\n", len(modules)))
		for _, m := range modules {
			b.WriteString(fmt.Sprintf("  %s\n", m[1]))
		}

	case "validate":
		var issues []string

		if len(resourceRe.FindAllStringSubmatch(content, -1)) == 0 {
			issues = append(issues, "No resource blocks found")
		}
		if len(providerRe.FindAllStringSubmatch(content, -1)) == 0 {
			issues = append(issues, "No provider blocks found")
		}

		unmatched := countUnmatchedBraces(content)
		if unmatched > 0 {
			issues = append(issues, fmt.Sprintf("Possible syntax issue: %d unmatched braces", unmatched))
		}

		if len(issues) == 0 {
			b.WriteString("Validation passed:\n")
			b.WriteString(fmt.Sprintf("  - %d .tf files scanned\n", len(tfFiles)))
			b.WriteString(fmt.Sprintf("  - %d resource(s) found\n", len(resourceRe.FindAllStringSubmatch(content, -1))))
			b.WriteString("  - No obvious syntax issues\n")
		} else {
			b.WriteString("Validation warnings:\n")
			for _, issue := range issues {
				b.WriteString(fmt.Sprintf("  - %s\n", issue))
			}
		}

	case "summary":
		resources := resourceRe.FindAllStringSubmatch(content, -1)
		providers := providerRe.FindAllStringSubmatch(content, -1)
		modules := moduleRe.FindAllStringSubmatch(content, -1)
		variables := variableRe.FindAllStringSubmatch(content, -1)
		outputs := outputRe.FindAllStringSubmatch(content, -1)
		datas := dataRe.FindAllStringSubmatch(content, -1)

		providerSet := make(map[string]bool)
		for _, p := range providers {
			providerSet[p[1]] = true
		}

		resourceByType := make(map[string]int)
		for _, r := range resources {
			resourceByType[r[1]]++
		}

		b.WriteString(fmt.Sprintf("Terraform Summary (%d files):\n\n", len(tfFiles)))
		b.WriteString(fmt.Sprintf("Providers: %d (%s)\n", len(providerSet), joinMapKeys(providerSet)))
		b.WriteString(fmt.Sprintf("Resources: %d\n", len(resources)))
		if len(resourceByType) > 0 {
			for t, c := range resourceByType {
				b.WriteString(fmt.Sprintf("  %s: %d\n", t, c))
			}
		}
		b.WriteString(fmt.Sprintf("Data sources: %d\n", len(datas)))
		b.WriteString(fmt.Sprintf("Modules: %d\n", len(modules)))
		b.WriteString(fmt.Sprintf("Variables: %d\n", len(variables)))
		b.WriteString(fmt.Sprintf("Outputs: %d\n", len(outputs)))
	}

	return b.String(), nil
}

func findTFFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".tf") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func countUnmatchedBraces(content string) int {
	count := 0
	for _, ch := range content {
		switch ch {
		case '{':
			count++
		case '}':
			count--
		}
	}
	if count < 0 {
		count = -count
	}
	return count
}

func joinMapKeys(m map[string]bool) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func NewBuiltinTerraformPlanTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolTerraformPlan,
			Desc:  "Analyze local .tf files: list resources, providers, modules, validate syntax. No terraform CLI required.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: resources, providers, modules, validate, summary", Required: false},
				"path":      {Type: einoschema.String, Desc: "Terraform directory (default: current)", Required: false},
			}),
		},
		execBuiltinTerraformPlan,
	)
}
