package workflow

import (
	"context"
	"sync"
	"time"

	"github.com/fisk086/sya/internal/config"
	"github.com/fisk086/sya/internal/logger"
)

type SandboxType string

const (
	SandboxTypeDocker       SandboxType = "docker"
	SandboxTypeKubernetes  SandboxType = "kubernetes"
	SandboxTypeNone        SandboxType = "none"
)

type Sandbox interface {
	Execute(ctx context.Context, language, code string, input map[string]any) (string, error)
	Type() SandboxType
}

var (
	globalSandbox Sandbox
	sandboxOnce   sync.Once
	sandboxCfg    *config.Settings
)

func InitSandbox(cfg *config.Settings) {
	sandboxOnce.Do(func() {
		sandboxCfg = cfg
		switch SandboxType(cfg.SandboxType) {
		case SandboxTypeDocker:
			logger.Info("sandbox: using docker", "images", cfg.SandboxDockerImages)
			globalSandbox = NewDockerSandbox(cfg)
		case SandboxTypeKubernetes:
			logger.Info("sandbox: using kubernetes (not implemented yet, fallback to docker)")
			globalSandbox = NewDockerSandbox(cfg)
		default:
			logger.Info("sandbox: using host runtime")
			globalSandbox = &HostRuntimeSandbox{}
		}
	})
}

func GetSandbox() Sandbox {
	if globalSandbox == nil {
		panic("sandbox not initialized, call InitSandbox first")
	}
	return globalSandbox
}

func ExecuteCode(ctx context.Context, language, code string, input map[string]any) (string, error) {
	return GetSandbox().Execute(ctx, language, code, input)
}

type HostRuntimeSandbox struct{}

func (s *HostRuntimeSandbox) Execute(ctx context.Context, language, code string, input map[string]any) (string, error) {
	return executeCodeRuntime(ctx, language, code, input)
}

func (s *HostRuntimeSandbox) Type() SandboxType {
	return SandboxTypeNone
}

func NewDockerSandbox(cfg *config.Settings) *DockerSandbox {
	timeout := time.Duration(cfg.SandboxTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	memoryMB := cfg.SandboxMemoryLimitMB
	if memoryMB <= 0 {
		memoryMB = 256
	}
	cpuPercent := cfg.SandboxCPUPercent
	if cpuPercent <= 0 {
		cpuPercent = 50
	}
	return &DockerSandbox{
		timeout:    timeout,
		memoryMB:   memoryMB,
		cpuPercent: cpuPercent,
		images:     parseDockerImages(cfg.SandboxDockerImages),
	}
}

type DockerSandbox struct {
	timeout    time.Duration
	memoryMB   int
	cpuPercent int
	images     map[string]string
}

func (s *DockerSandbox) Type() SandboxType {
	return SandboxTypeDocker
}

func (s *DockerSandbox) getImage(language string) string {
	if img, ok := s.images[language]; ok {
		return img
	}
	switch language {
	case "python":
		return "python:3.11-slim"
	case "javascript":
		return "node:22-slim"
	case "shell":
		return "bash:latest"
	default:
		return "python:3.11-slim"
	}
}