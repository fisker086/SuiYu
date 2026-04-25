package agent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

// usageTrackingChatModel is a decorator around eino ToolCallingChatModel: it does not change
// chat behavior; it only reads ResponseMeta.Usage from each Generate / Stream chunk when
// context was prepared with Runtime.withUsageTracking (facade, non-invasive to ReAct / routes).
type usageTrackingChatModel struct {
	inner model.ToolCallingChatModel
}

// WrapToolCallingModelWithUsageTracking returns a model that delegates to inner and records
// token usage via context. If inner is nil, returns nil.
func WrapToolCallingModelWithUsageTracking(inner model.ToolCallingChatModel) model.ToolCallingChatModel {
	if inner == nil {
		return nil
	}
	return &usageTrackingChatModel{inner: inner}
}

func (w *usageTrackingChatModel) Generate(ctx context.Context, input []*einoschema.Message, opts ...model.Option) (*einoschema.Message, error) {
	ensureOpenAICompatibleMessageContent(input)
	msg, err := w.inner.Generate(ctx, input, opts...)
	if msg != nil {
		usageAccumulateFromContext(ctx, msg)
	}
	return msg, err
}

func (w *usageTrackingChatModel) Stream(ctx context.Context, input []*einoschema.Message, opts ...model.Option) (*einoschema.StreamReader[*einoschema.Message], error) {
	ensureOpenAICompatibleMessageContent(input)
	sr, err := w.inner.Stream(ctx, input, opts...)
	if err != nil || sr == nil {
		return sr, err
	}
	return einoschema.StreamReaderWithConvert(sr, func(msg *einoschema.Message) (*einoschema.Message, error) {
		if msg != nil {
			usageAccumulateFromContext(ctx, msg)
		}
		return msg, nil
	}), nil
}

func (w *usageTrackingChatModel) WithTools(tools []*einoschema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := w.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &usageTrackingChatModel{inner: inner}, nil
}
