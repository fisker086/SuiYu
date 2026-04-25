package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/chathistory"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/notify"
	apischema "github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/workflow"
	"github.com/go-co-op/gocron/v2"
)

type Scheduler struct {
	mu           sync.RWMutex
	running      bool
	store        Storage
	runtime      *agent.Runtime
	scheduler    gocron.Scheduler
	jobMap       map[int64]gocron.Job
	auditLogger  *agent.AuditLogger
	embeddingDim int
	graphEngine  *workflow.GraphEngine
}

type Storage interface {
	ListSchedules() ([]*model.Schedule, error)
	GetSchedule(id int64) (*model.Schedule, error)
	UpdateSchedule(id int64, schedule *model.Schedule) (*model.Schedule, error)
	CreateScheduleExecution(exec *model.ScheduleExecution) (*model.ScheduleExecution, error)
	UpdateScheduleExecution(id int64, status string, result, err string, durationMs int64) error
	GetChannel(id int64) (*model.Channel, error)

	ListRecentSessionMessages(ctx context.Context, sessionID string, limit int) ([]apischema.ChatHistoryMessage, error)
	ListChatSessions(ctx context.Context, agentID int64, userID string, limit, offset int) ([]apischema.ChatSession, error)
	StoreMemory(ctx context.Context, agentID int64, userID, sessionID, role, content string, embedding []float32, extra map[string]any) error
	CreateChatSession(ctx context.Context, agentID int64, userID string, groupID int64) (*apischema.ChatSession, error)
	UpdateScheduleChatSessionID(scheduleID int64, chatSessionID string) error
}

// Enabled policy:
//   - Start / AddOrUpdateSchedule only register gocron jobs when schedule.Enabled is true.
//   - runJob always reloads the row from storage and skips execution when enabled is false (defense in depth).
//   - TriggerSchedule rejects disabled schedules before runJob (ErrScheduleDisabled).

// ErrScheduleDisabled is returned by TriggerSchedule when the task is turned off in the UI/API.
var ErrScheduleDisabled = errors.New("schedule is disabled")

func NewScheduler(store Storage, runtime *agent.Runtime, auditLogger *agent.AuditLogger, embeddingDim int, graphEngine *workflow.GraphEngine) *Scheduler {
	if embeddingDim <= 0 {
		embeddingDim = 1536
	}
	return &Scheduler{
		store:        store,
		runtime:      runtime,
		jobMap:       make(map[int64]gocron.Job),
		auditLogger:  auditLogger,
		embeddingDim: embeddingDim,
		graphEngine:  graphEngine,
	}
}

func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	s.scheduler = scheduler
	s.running = true

	schedules, err := s.store.ListSchedules()
	if err != nil {
		logger.Warn("failed to load schedules", "err", err)
		return nil
	}

	for _, sch := range schedules {
		if !sch.Enabled {
			continue
		}
		if err := s.scheduleJob(sch); err != nil {
			logger.Warn("failed to schedule job", "schedule_id", sch.ID, "err", err)
		}
	}

	scheduler.Start()
	logger.Info("scheduler started", "jobs", len(s.jobMap))
	return nil
}

func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.scheduler.Shutdown()
	s.running = false
	logger.Info("scheduler stopped")
	return nil
}

func (s *Scheduler) AddOrUpdateSchedule(schedule *model.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler not running")
	}

	if existing, ok := s.jobMap[schedule.ID]; ok {
		s.scheduler.RemoveJob(existing.ID())
		delete(s.jobMap, schedule.ID)
	}

	if schedule.Enabled {
		return s.scheduleJob(schedule)
	}
	return nil
}

func (s *Scheduler) RemoveSchedule(scheduleID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobMap[scheduleID]; ok {
		s.scheduler.RemoveJob(job.ID())
		delete(s.jobMap, scheduleID)
	}
	return nil
}

// triggerUserID: non-empty when API 手动触发（用 JWT 用户作为本次执行的归属）；cron 传空串则仅用 DB 中的 owner_user_id。
func (s *Scheduler) TriggerSchedule(ctx context.Context, scheduleID int64, triggerUserID string) error {
	schedule, err := s.store.GetSchedule(scheduleID)
	if err != nil {
		return fmt.Errorf("schedule not found: %d", scheduleID)
	}
	if !schedule.Enabled {
		return ErrScheduleDisabled
	}

	return s.runJob(ctx, schedule, triggerUserID)
}

// scheduleJob registers a gocron callback. Does not register when the task is disabled.
func (s *Scheduler) scheduleJob(schedule *model.Schedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule is nil")
	}
	if !schedule.Enabled {
		return fmt.Errorf("schedule is disabled")
	}

	scheduleCopy := schedule

	var jobDef gocron.JobDefinition
	var err error

	switch schedule.ScheduleKind {
	case "cron":
		jobDef = gocron.CronJob(schedule.CronExpr, false)
	case "at":
		t, err := time.Parse(time.RFC3339, schedule.At)
		if err != nil {
			return fmt.Errorf("invalid at time: %w", err)
		}
		jobDef = gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(t))
	case "every":
		jobDef = gocron.DurationJob(time.Duration(schedule.EveryMs) * time.Millisecond)
	default:
		return fmt.Errorf("unknown schedule kind: %s", schedule.ScheduleKind)
	}

	job, err := s.scheduler.NewJob(
		jobDef,
		gocron.NewTask(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := s.runJob(ctx, scheduleCopy, ""); err != nil {
				logger.Error("schedule job failed", "schedule_id", scheduleCopy.ID, "err", err)
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to add job: %w", err)
	}

	s.jobMap[schedule.ID] = job
	logger.Info("scheduled job", "schedule_id", schedule.ID, "name", schedule.Name, "kind", schedule.ScheduleKind)
	return nil
}

// runJob loads the latest schedule from storage and runs only if enabled is true.
// runAsUserID: 非空时优先作为本次执行的归属用户（手动触发）；空则仅用 schedule.OwnerUserID（定时触发）。
// Skipped runs produce no execution row and no notification (cron may still fire once if job removal lagged).
func (s *Scheduler) runJob(ctx context.Context, schedule *model.Schedule, runAsUserID string) error {
	sch, err := s.store.GetSchedule(schedule.ID)
	if err != nil {
		logger.Error("schedule reload failed", "schedule_id", schedule.ID, "err", err)
		return err
	}
	schedule = sch

	if !schedule.Enabled {
		logger.Info("schedule is disabled, skipping run", "schedule_id", schedule.ID, "name", schedule.Name)
		return nil
	}

	effectiveOwner := strings.TrimSpace(schedule.OwnerUserID)
	if ou := strings.TrimSpace(runAsUserID); ou != "" {
		effectiveOwner = ou
	}
	logger.Info("running scheduled task", "schedule_id", schedule.ID, "name", schedule.Name, "from_manual_trigger", strings.TrimSpace(runAsUserID) != "", "has_effective_owner", effectiveOwner != "")

	exec, err := s.store.CreateScheduleExecution(&model.ScheduleExecution{
		ScheduleID: schedule.ID,
		Status:     "running",
	})
	if err != nil {
		logger.Error("failed to create execution record", "err", err)
		return err
	}

	start := time.Now()

	result, err := s.executePrompt(ctx, schedule, effectiveOwner)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		s.store.UpdateScheduleExecution(exec.ID, "failed", "", err.Error(), durationMs)
		s.notifyScheduleResult(ctx, schedule, "failed", "", err.Error(), durationMs)
		logger.Error("schedule execution failed", "schedule_id", schedule.ID, "name", schedule.Name, "agent_id", schedule.AgentID, "execution_id", exec.ID, "session_target", schedule.SessionTarget, "duration_ms", durationMs, "err", err)
		if s.auditLogger != nil {
			uid, sid := scheduleAuditFields(schedule, effectiveOwner)
			s.auditLogger.Log(ctx, &model.AuditLog{
				UserID:     uid,
				SessionID:  sid,
				AgentID:    schedule.AgentID,
				ToolName:   "schedule",
				Action:     "execute",
				RiskLevel:  apischema.RiskLevelLow,
				Input:      schedule.Prompt,
				Error:      err.Error(),
				Status:     "failed",
				DurationMs: durationMs,
			})
		}
		return err
	}

	s.store.UpdateScheduleExecution(exec.ID, "success", result, "", durationMs)
	s.notifyScheduleResult(ctx, schedule, "success", result, "", durationMs)
	logger.Info("schedule execution completed", "schedule_id", schedule.ID, "duration_ms", durationMs)
	if s.auditLogger != nil {
		uid, sid := scheduleAuditFields(schedule, effectiveOwner)
		s.auditLogger.Log(ctx, &model.AuditLog{
			UserID:     uid,
			SessionID:  sid,
			AgentID:    schedule.AgentID,
			ToolName:   "schedule",
			Action:     "execute",
			RiskLevel:  apischema.RiskLevelLow,
			Input:      schedule.Prompt,
			Output:     result,
			Status:     "success",
			DurationMs: durationMs,
		})
	}
	return nil
}

// scheduleSessionUserID is used for agent_memory.user_id when no login owner (legacy main session key schedule:{id}).
const scheduleSessionUserID = "schedule"

// scheduleAuditFields fills audit_logs.user_id / session_id. effectiveUserID is the run归属（手动触发为 JWT 用户，定时为 OwnerUserID）。
func scheduleAuditFields(sch *model.Schedule, effectiveUserID string) (userID, sessionID string) {
	userID = strings.TrimSpace(effectiveUserID)
	if userID == "" {
		userID = strings.TrimSpace(sch.OwnerUserID)
	}
	sessionID = strings.TrimSpace(sch.ChatSessionID)
	if sessionID == "" && userID == "" {
		sessionID = fmt.Sprintf("schedule:%d", sch.ID)
	}
	return userID, sessionID
}

// logScheduleExecuteErr prints structured error logs when a schedule step fails (session_target + stage for定位).
func logScheduleExecuteErr(sch *model.Schedule, st, stage string, err error) {
	if err == nil {
		return
	}
	logger.Error("schedule execute failed", "schedule_id", sch.ID, "name", sch.Name, "agent_id", sch.AgentID, "session_target", st, "stage", stage, "err", err)
}

// ownerForRun 为本次执行归属用户：runJob 已合并「手动触发 JWT」与「定时任务 DB owner」。
func (s *Scheduler) executePrompt(ctx context.Context, sch *model.Schedule, ownerForRun string) (string, error) {
	// 代码执行
	if sch.CodeLanguage != "" {
		return s.executeCode(ctx, sch)
	}

	// 工作流执行
	if sch.WorkflowID > 0 {
		return s.executeWorkflow(ctx, sch, ownerForRun)
	}

	// Agent 执行
	return s.executeAgent(ctx, sch, ownerForRun)
}

func (s *Scheduler) executeCode(ctx context.Context, sch *model.Schedule) (string, error) {
	language := sch.CodeLanguage
	code := sch.Prompt

	var result string
	var err error

	switch language {
	case "python", "javascript", "shell":
		result, err = workflow.ExecuteCode(ctx, language, code, nil)
	default:
		err = fmt.Errorf("unsupported code language: %s", language)
	}

	if err != nil {
		logScheduleExecuteErr(sch, "code", "execute_failed", err)
		return "", err
	}

	return result, nil
}

func (s *Scheduler) executeWorkflow(ctx context.Context, sch *model.Schedule, ownerForRun string) (string, error) {
	if s.graphEngine == nil {
		err := fmt.Errorf("workflow engine not initialized")
		logScheduleExecuteErr(sch, "workflow", "engine_not_init", err)
		return "", err
	}

	userMsg := strings.TrimSpace(sch.Prompt)
	variables := map[string]any{
		"schedule_id":   sch.ID,
		"schedule_name": sch.Name,
		"owner":         ownerForRun,
	}

	result, err := s.graphEngine.Execute(ctx, sch.WorkflowID, userMsg, variables)
	if err != nil {
		logScheduleExecuteErr(sch, "workflow", "execute_failed", err)
		return "", err
	}

	if result == nil || result.Output == nil {
		return "", nil
	}

	if s, ok := result.Output.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", result.Output), nil
}

func (s *Scheduler) executeAgent(ctx context.Context, sch *model.Schedule, ownerForRun string) (string, error) {
	st := strings.ToLower(strings.TrimSpace(sch.SessionTarget))
	if st == "" || st == "new" {
		st = "main"
	}

	if _, ok := s.runtime.GetAgent(sch.AgentID); !ok {
		err := fmt.Errorf("agent not found: %d", sch.AgentID)
		logScheduleExecuteErr(sch, st, "agent_not_found", err)
		return "", err
	}

	// 仅发送定时任务里的用户提示词；Agent 系统提示词由 Runtime（ChatWithMemoryContextSchedule / Chat）注入，勿再拼进 user 消息以免重复耗 token。
	userMsg := strings.TrimSpace(sch.Prompt)

	memUserID := scheduleSessionUserID
	sessionID := ""

	owner := strings.TrimSpace(ownerForRun)
	if owner != "" {
		switch st {
		case "isolated":
			// 每次执行新建会话；结果写入该会话，并回写 chat_session_id 便于「最近一次」跳转
			sess, err := s.store.CreateChatSession(ctx, sch.AgentID, owner, 0)
			if err != nil {
				wrap := fmt.Errorf("schedule isolated: create chat session: %w", err)
				logScheduleExecuteErr(sch, st, "create_isolated_session", wrap)
				return "", wrap
			}
			if sess == nil {
				errNil := fmt.Errorf("schedule isolated: create chat session returned nil")
				logScheduleExecuteErr(sch, st, "create_isolated_session_nil", errNil)
				return "", errNil
			}
			sessionID = sess.SessionID
			memUserID = owner
			if err := s.store.UpdateScheduleChatSessionID(sch.ID, sessionID); err != nil {
				logger.Warn("schedule persist chat_session_id failed", "schedule_id", sch.ID, "err", err)
			} else {
				sch.ChatSessionID = sessionID
			}
		default:
			// main：写入该智能体下「最近更新」的一条会话（无则新建）
			sessions, err := s.store.ListChatSessions(ctx, sch.AgentID, owner, 1, 0)
			if err != nil {
				logger.Warn("schedule list chat sessions failed", "schedule_id", sch.ID, "err", err)
			}
			sid := ""
			if len(sessions) > 0 {
				sid = strings.TrimSpace(sessions[0].SessionID)
			}
			if sid == "" {
				sess, cerr := s.store.CreateChatSession(ctx, sch.AgentID, owner, 0)
				if cerr != nil {
					wrap := fmt.Errorf("schedule main: create chat session: %w", cerr)
					logScheduleExecuteErr(sch, st, "create_main_session", wrap)
					return "", wrap
				}
				if sess == nil {
					errNil := fmt.Errorf("schedule main: create chat session returned nil")
					logScheduleExecuteErr(sch, st, "create_main_session_nil", errNil)
					return "", errNil
				}
				sid = sess.SessionID
			}
			sessionID = sid
			memUserID = owner
			if err := s.store.UpdateScheduleChatSessionID(sch.ID, sid); err != nil {
				logger.Warn("schedule persist chat_session_id failed", "schedule_id", sch.ID, "err", err)
			} else {
				sch.ChatSessionID = sid
			}
		}
	} else if st == "main" {
		sessionID = fmt.Sprintf("schedule:%d", sch.ID)
		memUserID = scheduleSessionUserID
	} else if st == "isolated" {
		err := fmt.Errorf("schedule isolated requires owner_user_id (save the schedule while logged in)")
		logScheduleExecuteErr(sch, st, "isolated_no_owner", err)
		return "", err
	}

	logger.Info("schedule execute", "schedule_id", sch.ID, "session_target", st, "session_bound", sessionID != "")

	var history []*einoschema.Message
	if st == "main" && sessionID != "" {
		msgs, err := s.store.ListRecentSessionMessages(ctx, sessionID, 20)
		if err != nil {
			logger.Warn("schedule session history load failed", "schedule_id", sch.ID, "err", err)
		} else {
			history = chathistory.ToEinoMessages(msgs)
		}
	}

	var resp string
	var err error
	var assistantExtra map[string]any
	if sessionID != "" {
		var payloads []map[string]any
		resp, payloads, err = s.runtime.ChatWithMemoryContextSchedule(ctx, sch.AgentID, userMsg, "", "", "", sessionID, memUserID, history)
		if err == nil && len(payloads) > 0 {
			assistantExtra = map[string]any{"react_steps": payloads}
		}
	} else {
		resp, err = s.runtime.Chat(ctx, sch.AgentID, userMsg, "", "")
	}
	if err != nil {
		stage := "chat_with_memory"
		if sessionID == "" {
			stage = "chat"
		}
		logScheduleExecuteErr(sch, st, stage, err)
		return "", err
	}

	if sessionID != "" {
		vec := make([]float32, s.embeddingDim)
		if um := strings.TrimSpace(userMsg); um != "" {
			if err := s.store.StoreMemory(ctx, sch.AgentID, memUserID, sessionID, "user", um, vec, nil); err != nil {
				logger.Error("schedule store user turn failed", "schedule_id", sch.ID, "name", sch.Name, "agent_id", sch.AgentID, "session_target", st, "session_id", sessionID, "err", err)
			}
		}
		if strings.TrimSpace(resp) != "" {
			if err := s.store.StoreMemory(ctx, sch.AgentID, memUserID, sessionID, "assistant", resp, vec, assistantExtra); err != nil {
				logger.Error("schedule store assistant turn failed", "schedule_id", sch.ID, "name", sch.Name, "agent_id", sch.AgentID, "session_target", st, "session_id", sessionID, "err", err)
			}
		}
	}

	return resp, nil
}

// 与 Lark 卡片正文上限对齐（略留余量给元信息行）
const scheduleNotifyBodyMaxRunes = 26000

// finalTextForLarkNotify builds the success body for Lark: **only** the final user-visible reply, never react_steps.
// ReAct uses PolishReActUserVisibleTextForNotify (first "---" split) so multi-separator reports are not reduced to a footer-only line.
func (s *Scheduler) finalTextForLarkNotify(agentID int64, rawResult string) string {
	text := strings.TrimSpace(rawResult)
	if text == "" {
		return ""
	}
	ag, ok := s.runtime.GetAgent(agentID)
	if !ok || ag.RuntimeProfile == nil {
		return text
	}
	mode := strings.ToLower(strings.TrimSpace(ag.RuntimeProfile.ExecutionMode))
	if mode == agent.ExecutionModeReAct {
		out := agent.PolishReActUserVisibleTextForNotify(text)
		if strings.TrimSpace(out) == "" {
			return text
		}
		return out
	}
	return text
}

// notifyScheduleResult 推送执行摘要。正文仅含最终答复，不含思考步骤（react_steps 只进会话库，不进推送）。
// Lark：使用 card_title + Markdown 分区，避免整段被当成「一行标题」显得简陋；其它通道用分段纯文本。
func (s *Scheduler) notifyScheduleResult(ctx context.Context, schedule *model.Schedule, status, result, errText string, durationMs int64) {
	if schedule.ChannelID == nil || *schedule.ChannelID < 1 {
		return
	}
	ch, err := s.store.GetChannel(*schedule.ChannelID)
	if err != nil || ch == nil || !ch.IsActive {
		logger.Warn("schedule notify skipped", "schedule_id", schedule.ID, "channel_id", schedule.ChannelID, "err", err)
		return
	}

	cardTitle := truncateRunesForCard(fmt.Sprintf("定时任务 · %s", schedule.Name), 100)
	stZh := scheduleStatusLabelZh(status)
	durHuman := formatScheduleDurationHuman(durationMs)
	kind := strings.ToLower(strings.TrimSpace(ch.Kind))

	var payload string
	var extra map[string]string

	if status == "success" {
		body := truncateScheduleNotifyRunes(s.finalTextForLarkNotify(schedule.AgentID, result), scheduleNotifyBodyMaxRunes)
		if strings.TrimSpace(body) == "" {
			body = "（未生成文本摘要；若任务以工具结果为主，请查看审计日志或执行记录。）"
		}
		if kind == "lark" {
			// 代码/Shell 输出常含 * ` 等，飞书卡片 markdown 易解析失败，用代码块包裹
			if strings.TrimSpace(schedule.CodeLanguage) != "" {
				body = notify.WrapForLarkMarkdownFence(body)
			}
			payload = buildScheduleLarkMarkdown(stZh, durHuman, body)
			extra = mergeChannelExtra(ch.Extra, map[string]string{"card_title": cardTitle})
		} else {
			payload = buildSchedulePlainText(schedule.Name, stZh, durHuman, "执行结果", body)
			extra = ch.Extra
		}
	} else {
		errPart := truncateScheduleNotifyRunes(strings.TrimSpace(errText), scheduleNotifyBodyMaxRunes)
		if kind == "lark" {
			if strings.TrimSpace(errPart) == "" {
				errPart = "（无错误详情）"
			} else {
				errPart = notify.WrapForLarkMarkdownFence(errPart)
			}
			payload = buildScheduleLarkMarkdown(stZh, durHuman, errPart)
			extra = mergeChannelExtra(ch.Extra, map[string]string{"card_title": cardTitle})
		} else {
			payload = buildSchedulePlainText(schedule.Name, stZh, durHuman, "错误信息", errPart)
			extra = ch.Extra
		}
	}

	if err := notify.SendText(ctx, ch.Kind, ch.WebhookURL, ch.AppID, ch.AppSecret, extra, payload); err != nil {
		var chID int64
		if schedule.ChannelID != nil {
			chID = *schedule.ChannelID
		}
		logger.Error("schedule notify failed", "schedule_id", schedule.ID, "name", schedule.Name, "channel_id", chID, "err", err)
	}
}

func mergeChannelExtra(base map[string]string, add map[string]string) map[string]string {
	if len(base) == 0 && len(add) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(add))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range add {
		out[k] = v
	}
	return out
}

func truncateRunesForCard(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

func scheduleStatusLabelZh(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "running":
		return "运行中"
	default:
		if strings.TrimSpace(status) == "" {
			return "—"
		}
		return status
	}
}

func formatScheduleDurationHuman(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	if ms >= 3600000 {
		h := ms / 3600000
		m := (ms % 3600000) / 60000
		return fmt.Sprintf("%d 小时 %d 分", h, m)
	}
	if ms >= 60000 {
		m := ms / 60000
		sec := (ms % 60000) / 1000
		return fmt.Sprintf("%d 分 %d 秒", m, sec)
	}
	if ms >= 1000 {
		return fmt.Sprintf("%.1f 秒", float64(ms)/1000)
	}
	return fmt.Sprintf("%d ms", ms)
}

// buildScheduleLarkMarkdown 供飞书卡片 markdown 区：元信息 + 正文。不再插入 --- 分隔线（部分 stderr 与飞书解析器组合会触发 11311）。
func buildScheduleLarkMarkdown(statusZh, durHuman, sectionBody string) string {
	meta := fmt.Sprintf("**状态** · %s  ·  **耗时** · %s", statusZh, durHuman)
	sectionBody = strings.TrimSpace(sectionBody)
	if sectionBody == "" {
		sectionBody = "（无）"
	}
	return meta + "\n\n" + sectionBody
}

func buildSchedulePlainText(taskName, statusZh, durHuman, sectionTitle, sectionBody string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "【%s】\n", taskName)
	fmt.Fprintf(&b, "状态：%s\n", statusZh)
	fmt.Fprintf(&b, "耗时：%s\n", durHuman)
	b.WriteString("\n────────────────────────\n")
	fmt.Fprintf(&b, "%s\n", sectionTitle)
	b.WriteString("────────────────────────\n\n")
	b.WriteString(strings.TrimSpace(sectionBody))
	return b.String()
}

func truncateScheduleNotifyRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "\n\n…(truncated)"
}
