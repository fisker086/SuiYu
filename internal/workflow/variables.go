package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

type VariableContext struct {
	SystemVariables map[string]any // sys.* 系统变量
	InputVariables  map[string]any // start 节点的输入参数
	GlobalVariables map[string]any // 用户定义的全局变量
	NodeOutputs     map[string]any // 节点ID -> 输出
}

func NewVariableContext() *VariableContext {
	return &VariableContext{
		SystemVariables: make(map[string]any),
		InputVariables:  make(map[string]any),
		GlobalVariables: make(map[string]any),
		NodeOutputs:     make(map[string]any),
	}
}

func (vc *VariableContext) SetSystemVariable(key string, value any) {
	vc.SystemVariables[key] = value
}

func (vc *VariableContext) SetInputVariable(key string, value any) {
	vc.InputVariables[key] = value
}

func (vc *VariableContext) SetGlobalVariable(key string, value any) {
	vc.GlobalVariables[key] = value
}

func (vc *VariableContext) SetNodeOutput(nodeID string, output any) {
	vc.NodeOutputs[nodeID] = output
}

func (vc *VariableContext) GetVariable(key string) (any, bool) {
	// 优先级: 系统变量 > 输入变量 > 全局变量 > 节点输出

	// 1. 系统变量 (sys.xxx)
	if strings.HasPrefix(key, "sys.") {
		if val, ok := vc.SystemVariables[key]; ok {
			return val, true
		}
		return nil, false
	}

	// 2. 输入变量 (start 节点定义的参数)
	if val, ok := vc.InputVariables[key]; ok {
		return val, true
	}

	// 3. 全局变量
	if val, ok := vc.GlobalVariables[key]; ok {
		return val, true
	}

	// 4. 节点输出 (支持 node_id.field 格式)
	if val, ok := vc.NodeOutputs[key]; ok {
		return val, true
	}

	return nil, false
}

var templateVarRegex = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func ResolveTemplate(tmpl string, ctx *VariableContext) string {
	if tmpl == "" {
		return ""
	}

	return templateVarRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := templateVarRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varPath := strings.TrimSpace(submatches[1])
		parts := strings.Split(varPath, ".")

		// 获取基础值
		baseKey := parts[0]
		val, ok := ctx.GetVariable(baseKey)
		if !ok {
			return match // 变量不存在，保留原样
		}

		// 访问嵌套字段
		for i := 1; i < len(parts) && val != nil; i++ {
			val = getNestedValue(val, parts[i])
		}

		if val == nil {
			return match
		}

		// 转换为字符串
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	})
}

func getNestedValue(data any, field string) any {
	switch v := data.(type) {
	case map[string]any:
		return v[field]
	case []any:
		if idx := parseArrayIndex(field, len(v)); idx >= 0 && idx < len(v) {
			return v[idx]
		}
	}
	return nil
}

func parseArrayIndex(s string, length int) int {
	if s == "first" {
		return 0
	}
	if s == "last" {
		if length > 0 {
			return length - 1
		}
		return -1
	}
	n := 0
	negative := false
	for i, c := range s {
		if c == '-' && i == 0 {
			negative = true
			continue
		}
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if negative {
		n = -n
	}
	if n >= 0 && n < length {
		return n
	}
	return -1
}

func ResolveTemplateForCode(code string, ctx *VariableContext) string {
	// 对于代码块，替换所有变量引用
	return ResolveTemplate(code, ctx)
}
