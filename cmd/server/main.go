package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	_ "github.com/fisk086/sya/docs"
	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/authprovider"
	"github.com/fisk086/sya/internal/config"
	"github.com/fisk086/sya/internal/controller"
	"github.com/fisk086/sya/internal/embedding"
	"github.com/fisk086/sya/internal/gateway"
	"github.com/fisk086/sya/internal/immanager"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/mcp"
	"github.com/fisk086/sya/internal/memory"
	"github.com/fisk086/sya/internal/memory/pgvector"
	"github.com/fisk086/sya/internal/scheduler"
	agentSchema "github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/skills"
	"github.com/fisk086/sya/internal/storage"
	"github.com/fisk086/sya/internal/webui"
	"github.com/fisk086/sya/internal/workflow"
	"github.com/hertz-contrib/cors"
	"github.com/hertz-contrib/swagger"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
)

// @title Eino Agent API
// @version 1.0
// @description AI Agent configuration platform with Skills and MCP support
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@aiops.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT access token (Authorization: Bearer plus space plus token)
func main() {
	_ = godotenv.Load()
	logger.Init()

	settings := config.Load()
	workflow.InitSandbox(settings)

	if settings.JWTSecretKey == "" {
		logger.Fatal("JWT_SECRET_KEY environment variable is required")
	}
	if settings.DisableLoginCaptcha {
		logger.Warn("DISABLE_LOGIN_CAPTCHA is enabled; login captcha verification is off (not for production)")
	}

	mcpClient := mcp.NewClient()

	skillLoader := skills.NewLoader(settings.SkillsDir)
	skillRegistry := skills.NewRegistry()

	if loadedSkills, err := skillLoader.LoadAll(); err == nil {
		for _, s := range loadedSkills {
			skillRegistry.Register(s)
			logger.Info("loaded skill", "name", s.Name)
		}
	}

	store := storage.NewInMemoryStorage()
	var pgStore storage.Storage
	var embedService *embedding.Service
	var gormDB *storage.GORMDB

	if settings.DatabaseURL != "" {
		ctx := context.Background()
		storage.LogDatabaseTarget(settings.DatabaseURL)
		logger.Info("database init", "step", "connect_postgres_with_retry")
		pg, err := storage.ConnectPostgresWithRetry(ctx, settings.DatabaseURL, settings.EmbeddingDimension)
		if err != nil {
			logger.Fatal("failed to connect to PostgreSQL", "err", err)
		}

		logger.Info("database init", "step", "migrate_sql")
		if err := pg.Migrate(ctx); err != nil {
			logger.Fatal("failed to run migrations", "err", err)
		}
		logger.Info("connected to PostgreSQL and ran migrations")
		pgStore = pg

		logger.Info("database init", "step", "gorm_open")
		gormDB, err = storage.NewGORMDB(settings.DatabaseURL)
		if err != nil {
			logger.Fatal("failed to initialize GORM", "err", err)
		}
		if err := gormDB.AutoMigrate(); err != nil {
			logger.Fatal("failed to auto-migrate GORM models", "err", err)
		}
		if err := gormDB.SeedDefaultAdmin(); err != nil {
			logger.Warn("failed to seed default admin", "err", err)
		}
		if err := gormDB.ApplyAdminWhitelist(settings.AdminWhitelist); err != nil {
			logger.Warn("failed to apply ADMIN_WHITELIST", "err", err)
		}
		defer func() {
			if sqlDB, err := gormDB.DB.DB(); err == nil {
				sqlDB.Close()
			}
			pg.Close()
		}()
	}

	if settings.EmbeddingAPIKey != "" {
		embedService = embedding.NewService(
			settings.EmbeddingAPIKey,
			settings.EmbeddingModel,
			settings.EmbeddingBaseURL,
			settings.EmbeddingDimension,
		)
		logger.Info("embedding service initialized", "model", settings.EmbeddingModel)
	}

	chatModel, err := newChatModel(settings)
	if err != nil {
		logger.Fatal("failed to create chat model", "err", err)
	}
	chatModel = agent.WrapToolCallingModelWithUsageTracking(chatModel)

	activeStore := pgStore
	if activeStore == nil {
		activeStore = store
		logger.Info("using in-memory storage (set DATABASE_URL to use PostgreSQL)")
	}

	runtime := agent.NewRuntimeWithSkill(chatModel, mcpClient, skillLoader, skillRegistry, activeStore)
	runtime.SetDefaultChatModelName(settings.EffectiveChatModel())

	createDemoAgent(activeStore, runtime)
	initDependencies(activeStore, runtime)
	skillService := service.NewSkillService(activeStore)
	if n := skillService.SyncBuiltinSkills(skillRegistry, settings.SkillsDir); n > 0 {
		logger.Info("synced builtin skills from skills directory", "created", n)
	}
	syncAgentsToRuntime(activeStore, runtime)

	// IM bot initialization (supports lark, dingtalk, wecom, etc.)
	imManager := immanager.NewIMManager()
	if err := imManager.ScanAndRegister(activeStore, runtime); err != nil {
		logger.Warn("failed to scan and register im bots", "err", err)
	}

	// Services
	agentService := service.NewAgentService(activeStore)
	mcpService := service.NewMCPService(activeStore, mcpClient)
	channelService := service.NewChannelService(activeStore)
	workflowService := service.NewWorkflowService(activeStore)
	memProvider := buildMemoryProvider(settings, embedService, activeStore)
	if memProvider != nil {
		logger.Info("long-term memory provider enabled", "provider", settings.MemoryProvider)
	}
	chatService := service.NewChatService(runtime, activeStore, memProvider, embedService, workflowService, settings.EmbeddingDimension)

	// Initialize group coordinator for multi-agent group chat capability discovery
	groupCoordinator := agent.NewGroupCoordinator(activeStore, runtime, agent.NewRuntimeAgentCaller(runtime))
	runtime.SetGroupCoordinator(groupCoordinator)

	// RBAC Service
	rbacSvc := service.NewRBACService(activeStore)

	// Message Router & Service
	messageRouter := gateway.NewMessageRouter(activeStore, runtime, 1024)
	messageRouter.InitAgents()
	messageService := service.NewMessageService(messageRouter)

	// Graph Workflow Engine & Service
	graphEngine := workflow.NewGraphEngine(runtime, activeStore)
	graphWorkflowService := service.NewGraphWorkflowService(graphEngine, activeStore, activeStore, runtime.AuditLogger())

	// Schedule Service (依赖 graphEngine)
	scheduleScheduler := scheduler.NewScheduler(activeStore, runtime, runtime.AuditLogger(), settings.EmbeddingDimension, graphEngine)
	scheduleService := service.NewScheduleService(activeStore, scheduleScheduler, activeStore, activeStore)
	if err := scheduleService.StartScheduler(); err != nil {
		logger.Warn("failed to start schedule scheduler", "err", err)
	}

	// Controllers
	var chatUserStore storage.UserStore
	if gormDB != nil {
		chatUserStore = gormDB.UserStore
	}
	jwtCfg := auth.NewJWTConfig(settings.JWTSecretKey)
	authCtrl := controller.NewAuthController(gormDB.UserStore, jwtCfg, settings.DisableLoginCaptcha, settings.AuthType)
	var ssoCtrl *controller.SSOController
	if gormDB != nil {
		ssoReg := authprovider.NewRegistry(authprovider.DefaultProviders(gormDB.UserStore, jwtCfg, settings)...)
		ssoCtrl = controller.NewSSOController(ssoReg)
	}
	agentCtrl := controller.NewAgentController(agentService, chatService, runtime, jwtCfg, chatUserStore, rbacSvc)
	agentCtrl.SetIMManager(imManager)
	skillCtrl := controller.NewSkillController(skillService, skillRegistry, settings.SkillsDir)
	mcpCtrl := controller.NewMCPController(mcpService)
	channelCtrl := controller.NewChannelController(channelService)
	workflowCtrl := controller.NewWorkflowController(workflowService)
	rbacCtrl := controller.NewRBACController(rbacSvc, gormDB.UserStore, jwtCfg)
	messageCtrl := controller.NewMessageController(messageService)
	graphWorkflowCtrl := controller.NewGraphWorkflowController(graphWorkflowService)
	chatCtrl := controller.NewChatController(chatService, jwtCfg, chatUserStore, settings.UploadDir, rbacSvc)
	scheduleCtrl := controller.NewScheduleController(scheduleService, jwtCfg, chatUserStore)
	var auditUserLookup storage.UserStore
	if gormDB != nil {
		auditUserLookup = gormDB.UserStore
	}
	auditCtrl := controller.NewAuditController(service.NewAuditService(activeStore, auditUserLookup, activeStore), jwtCfg, auditUserLookup)
	versionCtrl := controller.NewVersionController(settings.GitHubRepo)
	approvalCtrl := controller.NewApprovalController(service.NewApprovalService(activeStore, activeStore), rbacSvc, jwtCfg, chatUserStore)

	var usageCtrl *controller.TokenUsageController
	if gormDB != nil {
		usageSvc := service.NewTokenUsageService(gormDB.DB, settings.EffectiveChatModel())
		runtime.SetTokenUsageSink(usageSvc)
		usageCtrl = controller.NewTokenUsageController(usageSvc, jwtCfg, func(userID int64) (*auth.User, error) {
			user, err := gormDB.UserStore.GetUserByID(userID)
			if err != nil {
				return nil, err
			}
			return &auth.User{
				ID:       user.ID,
				Username: user.Username,
				Email:    user.Email,
				Status:   string(user.Status),
				IsAdmin:  user.IsAdmin,
			}, nil
		})
	}

	h := server.New(
		server.WithHostPorts(fmt.Sprintf(":%d", settings.ServerPort)),
	)

	// Middleware order: outer recovery first, then CORS (same as server.Default + custom chain).
	h.Use(recovery.Recovery(recovery.WithRecoveryHandler(func(c context.Context, ctx *app.RequestContext, err interface{}, stack []byte) {
		logger.Error("handler panic", "recover", err, "stack", string(stack))
		ctx.AbortWithStatus(consts.StatusInternalServerError)
	})))

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsCfg.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Requested-With"}
	corsCfg.ExposeHeaders = []string{"Content-Length", "Content-Type"}
	corsCfg.MaxAge = 86400 * time.Second
	h.Use(cors.New(corsCfg))

	// Serve uploaded files: Hertz Static() joins Root + full ctx.Path(), so requests would wrongly map to
	// uploads/api/v1/chat/files/... . Strip the four URL segments (/api /v1 /chat /files) so paths resolve to uploads/<uid>/<file>.
	h.StaticFS("/api/v1/chat/files", &app.FS{
		Root:        settings.UploadDir,
		PathRewrite: app.NewPathSlashesStripper(4),
	})

	// Register routes
	authCtrl.RegisterRoutes(h)
	if ssoCtrl != nil {
		ssoCtrl.RegisterRoutes(h)
	}
	agentCtrl.RegisterRoutes(h)
	skillCtrl.RegisterRoutes(h)
	mcpCtrl.RegisterRoutes(h)
	channelCtrl.RegisterRoutes(h)
	workflowCtrl.RegisterRoutes(h)
	rbacCtrl.RegisterRoutes(h)
	chatCtrl.RegisterRoutes(h)
	messageCtrl.RegisterRoutes(h)
	graphWorkflowCtrl.RegisterRoutes(h)
	scheduleCtrl.RegisterRoutes(h)
	auditCtrl.RegisterRoutes(h)
	approvalCtrl.RegisterRoutes(h)
	if usageCtrl != nil {
		usageCtrl.RegisterRoutes(h)
	}
	versionCtrl.RegisterRoutes(h)

	// Lark bot controller
	larkBotCtrl := controller.NewLarkBotController(agentService)
	larkBotCtrl.RegisterRoutes(h)

	// Telegram bot controller
	telegramBotCtrl := controller.NewTelegramBotController(agentService)
	telegramBotCtrl.RegisterRoutes(h)

	h.GET("/swagger/*filepath", swagger.WrapHandler(swaggerFiles.Handler))

	webui.Register(h)

	logger.Info("starting server", "addr", fmt.Sprintf(":%d", settings.ServerPort))
	logger.Info("swagger UI", "url", fmt.Sprintf("http://localhost:%d/swagger/index.html", settings.ServerPort))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server")
		scheduleService.StopScheduler()
		imManager.StopAll()
		h.Shutdown(ctx)
	}()

	imManager.StartAll(ctx)
	logger.Info("im manager started")

	h.Spin()
}

type einoChatModel struct {
	openaiModel *openai.ChatModel
	arkModel    *ark.ChatModel
}

func newChatModel(settings *config.Settings) (model.ToolCallingChatModel, error) {
	modelType := settings.ModelType

	switch modelType {
	case "ark":
		arkModel, err := ark.NewChatModel(context.Background(), &ark.ChatModelConfig{
			APIKey:  settings.ArkAPIKey,
			Model:   settings.ArkModel,
			BaseURL: settings.ArkBaseURL,
		})
		if err != nil {
			return nil, err
		}
		return &einoChatModel{arkModel: arkModel}, nil
	default:
		openAIModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
			APIKey:  settings.OpenAIAPIKey,
			Model:   settings.OpenAIModel,
			BaseURL: settings.OpenAIBaseURL,
			ByAzure: settings.OpenAIAzure,
		})
		if err != nil {
			return nil, err
		}
		return &einoChatModel{openaiModel: openAIModel}, nil
	}
}

func (m *einoChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	var msg *schema.Message
	var err error
	if m.openaiModel != nil {
		msg, err = m.openaiModel.Generate(ctx, messages, opts...)
	} else if m.arkModel != nil {
		msg, err = m.arkModel.Generate(ctx, messages, opts...)
	} else {
		return nil, fmt.Errorf("no model available")
	}
	return msg, wrapLLMUpstreamError(err)
}

func (m *einoChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	var sr *schema.StreamReader[*schema.Message]
	var err error
	if m.openaiModel != nil {
		sr, err = m.openaiModel.Stream(ctx, messages, opts...)
	} else if m.arkModel != nil {
		sr, err = m.arkModel.Stream(ctx, messages, opts...)
	} else {
		return nil, fmt.Errorf("no model available")
	}
	return sr, wrapLLMUpstreamError(err)
}

func (m *einoChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if m.openaiModel != nil {
		return m.openaiModel.WithTools(tools)
	} else if m.arkModel != nil {
		return m.arkModel.WithTools(tools)
	}
	return nil, fmt.Errorf("no model available")
}

// wrapLLMUpstreamError turns opaque JSON-parse failures (HTML bodies from 403/Cloudflare/WAF) into a short, actionable message.
func wrapLLMUpstreamError(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	htmlLikely := strings.Contains(s, "invalid character '<'") ||
		strings.Contains(s, "<!DOCTYPE") ||
		(len(s) > 1800 && strings.Contains(strings.ToLower(s), "<html"))
	if !htmlLikely {
		return err
	}
	msg := "LLM upstream returned HTML instead of JSON"
	if strings.Contains(s, "403") {
		msg += " (HTTP 403)"
	}
	msg += ". Cloudflare or WAF often blocks server-side API calls from your egress IP. Allowlist that IP, use a provider API host without browser challenges, or verify OPENAI_BASE_URL and OPENAI_API_KEY."
	return errors.New(msg)
}

// syncAgentsToRuntime registers every agent from persistent storage into the in-memory Runtime.
// Chat and tools resolve agents only via Runtime; without this, a server restart would leave DB agents
// visible in GET /agents but unusable for POST /chat ("agent not found").
func syncAgentsToRuntime(store storage.Storage, runtime *agent.Runtime) {
	list, err := store.ListAgents()
	if err != nil {
		logger.Warn("failed to list agents for runtime sync", "err", err)
		return
	}
	for _, a := range list {
		if a == nil {
			continue
		}
		full, err := store.GetAgent(a.ID)
		if err != nil || full == nil {
			logger.Warn("skip agent runtime sync", "agent_id", a.ID, "err", err)
			continue
		}
		runtime.RegisterAgent(full)
	}
	logger.Info("runtime agent registry synced from storage", "count", len(list))
}

func buildMemoryProvider(settings *config.Settings, embed *embedding.Service, store storage.Storage) memory.Provider {
	if embed == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(settings.MemoryProvider)) {
	case "", "pgvector":
		return pgvector.New(embed, store, pgvector.Options{RetrieveTopK: settings.MemoryRetrieveTopK})
	case "none", "off", "disabled":
		return nil
	default:
		logger.Warn("unknown MEMORY_PROVIDER; long-term memory disabled", "provider", settings.MemoryProvider)
		return nil
	}
}

func createDemoAgent(store storage.Storage, runtime *agent.Runtime) {
	// Only seed when there are no agents yet. Otherwise each server restart on PostgreSQL
	// would INSERT another duplicate "Demo Agent" row (agents.name is not unique).
	existing, err := store.ListAgents()
	if err != nil {
		logger.Warn("failed to list agents before demo seed", "err", err)
		return
	}
	if len(existing) > 0 {
		logger.Info("skip default demo agent seed; agents already exist", "count", len(existing))
		return
	}

	agentReq := &agentSchema.CreateAgentRequest{
		Name:        "Demo Agent",
		Description: "A demo agent with skills and MCP tools",
		Category:    "general",
		RuntimeProfile: &agentSchema.RuntimeProfile{
			SourceAgent:   "general_chat",
			Archetype:     "assistant",
			Role:          "You are a helpful AI assistant",
			Goal:          "Help users with their tasks",
			Backstory:     "You are an AI agent designed to assist users",
			LlmModel:      "",
			Temperature:   0.7,
			StreamEnabled: true,
			MemoryEnabled: false,
			SkillIDs:      []string{},
			MCPConfigIDs:  []int64{},
		},
	}

	createdAgent, err := store.CreateAgent(agentReq)
	if err != nil {
		logger.Error("failed to create demo agent", "err", err)
		return
	}

	fullAgent, _ := store.GetAgent(createdAgent.ID)
	if fullAgent != nil {
		runtime.RegisterAgent(fullAgent)
		logger.Info("created demo agent", "agent_id", createdAgent.ID)
	}
}

func initDependencies(store storage.Storage, runtime *agent.Runtime) {
	skills.SetLearningStore(store)
}
