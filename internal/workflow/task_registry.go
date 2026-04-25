package workflow

import (
	"context"
	"fmt"
	"sync"
)

type TaskType string

const (
	TaskTypeStart     TaskType = "start"
	TaskTypeEnd       TaskType = "end"
	TaskTypeAgent     TaskType = "agent"
	TaskTypeLLM       TaskType = "llm"
	TaskTypeTool      TaskType = "tool"
	TaskTypeHTTP      TaskType = "http"
	TaskTypeCode      TaskType = "code"
	TaskTypeCondition TaskType = "condition"
	TaskTypeKnowledge TaskType = "knowledge"
	TaskTypeTemplate  TaskType = "template"
	TaskTypeVariable  TaskType = "variable"
	TaskTypeMerge     TaskType = "merge"
	TaskTypeLoop      TaskType = "loop"
	TaskTypeParallel  TaskType = "parallel"
	TaskTypeBranch    TaskType = "branch"
	TaskTypeWait      TaskType = "wait"
)

type TaskInput struct {
	NodeID      string
	NodeLabel   string
	Config      map[string]any
	Variables   map[string]any
	NodeOutputs map[string]any
	UserMessage string
	VarContext  *VariableContext
}

type TaskOutput struct {
	Data  map[string]any
	Error string
}

type TaskFunc func(ctx context.Context, input *TaskInput) (*TaskOutput, error)

type TaskDefinition struct {
	Type         TaskType
	Name         string
	Description  string
	Icon         string
	Color        string
	Category     string
	Execute      TaskFunc
	ConfigSchema map[string]TaskField
}

type TaskField struct {
	Key         string
	Label       string
	Type        string
	Required    bool
	Default     any
	Description string
	Options     []string
}

var (
	tasksMu sync.RWMutex
	tasks   = make(map[TaskType]*TaskDefinition)
)

func RegisterTask(def *TaskDefinition) {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	tasks[def.Type] = def
}

func GetTask(t TaskType) (*TaskDefinition, bool) {
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	def, ok := tasks[t]
	return def, ok
}

func ListTasks() []*TaskDefinition {
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	result := make([]*TaskDefinition, 0, len(tasks))
	for _, def := range tasks {
		result = append(result, def)
	}
	return result
}

func ListTasksByCategory(category string) []*TaskDefinition {
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	var result []*TaskDefinition
	for _, def := range tasks {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

func ExecuteTask(ctx context.Context, taskType TaskType, input *TaskInput) (*TaskOutput, error) {
	def, ok := GetTask(taskType)
	if !ok {
		return nil, fmt.Errorf("task type not registered: %s", taskType)
	}
	if def.Execute == nil {
		return nil, fmt.Errorf("task %s has no executor", taskType)
	}
	return def.Execute(ctx, input)
}
