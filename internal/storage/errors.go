package storage

import "errors"

// ErrAgentNotFound is returned when an agent id does not exist.
var ErrAgentNotFound = errors.New("agent not found")

// ErrSessionNotFound is returned when a chat session id does not exist.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionForbidden is returned when the session exists but does not belong to the caller.
var ErrSessionForbidden = errors.New("session forbidden")

// ErrWorkflowNotFound is returned when a workflow id or key does not exist.
var ErrWorkflowNotFound = errors.New("workflow not found")

// ErrMCPConfigNotFound is returned when an MCP config id does not exist.
var ErrMCPConfigNotFound = errors.New("mcp config not found")

// ErrChannelNotFound is returned when a channel id does not exist.
var ErrChannelNotFound = errors.New("channel not found")
