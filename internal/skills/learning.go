package skills

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/schema"
)

const toolLearning = "builtin_learning"

type LearningStore interface {
	CreateLearning(req *schema.CreateLearningRequest) (*schema.Learning, error)
	ListLearnings(userID *int64) ([]*schema.Learning, error)
	GetLearning(userID *int64, errorType string) (*schema.Learning, error)
}

var learningStore LearningStore

func SetLearningStore(store LearningStore) {
	learningStore = store
}

func execBuiltinLearning(_ context.Context, in map[string]any) (string, error) {
	if learningStore == nil {
		return "", fmt.Errorf("learning store not configured")
	}

	op := strArg(in, "operation", "op", "action")
	userIDStr := strArg(in, "user_id")
	var userID *int64
	if userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid user_id: %w", err)
		}
		userID = &id
	}

	switch op {
	case "add", "create", "record":
		return handleAddLearning(in, userID)
	case "list", "get_all":
		return handleListLearnings(userID)
	case "get":
		errorType := strArg(in, "error_type")
		if errorType == "" {
			return "", fmt.Errorf("missing error_type")
		}
		return handleGetLearning(userID, errorType)
	default:
		return fmt.Sprintf(`Supported operations:
add: Record a learning (error_type, context, root_cause, fix, lesson)
list: List all learnings for user or global
get: Get specific learning by error_type`), nil
	}
}

func handleAddLearning(in map[string]any, userID *int64) (string, error) {
	req := &schema.CreateLearningRequest{
		UserID:    userID,
		ErrorType: strArg(in, "error_type"),
		Context:   strArg(in, "context"),
		RootCause: strArg(in, "root_cause"),
		Fix:       strArg(in, "fix"),
		Lesson:    strArg(in, "lesson"),
	}
	if req.ErrorType == "" {
		return "", fmt.Errorf("missing error_type")
	}

	l, err := learningStore.CreateLearning(req)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Learning recorded: ID=%d, ErrorType=%s, Times=%d\nLesson: %s",
		l.ID, l.ErrorType, l.Times, l.Lesson), nil
}

func handleListLearnings(userID *int64) (string, error) {
	list, err := learningStore.ListLearnings(userID)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "No learnings yet", nil
	}
	result := "Learnings:\n"
	for _, l := range list {
		scope := "global"
		if l.UserID != nil {
			scope = fmt.Sprintf("user:%d", *l.UserID)
		}
		result += fmt.Sprintf("- [%s] %s (x%d): %s\n", scope, l.ErrorType, l.Times, l.Lesson)
	}
	return result, nil
}

func handleGetLearning(userID *int64, errorType string) (string, error) {
	l, err := learningStore.GetLearning(userID, errorType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ErrorType: %s\nContext: %s\nRootCause: %s\nFix: %s\nLesson: %s\nTimes: %d",
		l.ErrorType, l.Context, l.RootCause, l.Fix, l.Lesson, l.Times), nil
}

func NewBuiltinLearningTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolLearning,
			Desc:  "Record and retrieve learnings from errors and corrections",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: add, list, get", Required: true},
				"user_id":    {Type: einoschema.String, Desc: "User ID (optional, nil=global)", Required: false},
				"error_type": {Type: einoschema.String, Desc: "Error type for add/get", Required: false},
				"context":    {Type: einoschema.String, Desc: "What happened", Required: false},
				"root_cause": {Type: einoschema.String, Desc: "Why it happened", Required: false},
				"fix":        {Type: einoschema.String, Desc: "What was done to fix", Required: false},
				"lesson":     {Type: einoschema.String, Desc: "Lesson learned", Required: false},
			}),
		},
		execBuiltinLearning,
	)
}
