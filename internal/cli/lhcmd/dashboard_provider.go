package lhcmd

import (
	"strings"
	"sync"
	"time"

	"github.com/yurika0211/luckyagent/internal/agent"
	"github.com/yurika0211/luckyagent/internal/config"
	"github.com/yurika0211/luckyagent/internal/cron"
	"github.com/yurika0211/luckyagent/internal/gateway"
	"github.com/yurika0211/luckyagent/internal/memory"
	"github.com/yurika0211/luckyagent/internal/server/dashboard"
	"github.com/yurika0211/luckyagent/internal/tool"
)

var (
	replDashboardMu sync.Mutex
	replDashboard   *dashboard.Dashboard
)

type replDashboardProvider struct {
	agent *agent.Agent
}

func ensureREPLDashboard(a *agent.Agent, addr string) (*dashboard.Dashboard, bool) {
	replDashboardMu.Lock()
	defer replDashboardMu.Unlock()

	if replDashboard != nil {
		return replDashboard, false
	}

	cfg := dashboard.Config{Addr: addr}
	replDashboard = dashboard.New(cfg)
	if a != nil {
		replDashboard.AddProvider(&replDashboardProvider{agent: a})
	}
	return replDashboard, true
}

func getREPLDashboard() *dashboard.Dashboard {
	replDashboardMu.Lock()
	defer replDashboardMu.Unlock()
	return replDashboard
}

func (p *replDashboardProvider) DashboardData() map[string]interface{} {
	data := map[string]interface{}{
		"active_profile": "repl",
	}
	if p == nil || p.agent == nil {
		return data
	}

	cfg := p.agent.Config().Get()
	homeDir := p.agent.Config().HomeDir()
	data["provider"] = cfg.Provider
	data["model"] = cfg.Model
	data["stream_mode"] = cfg.StreamMode
	data["soul_path"] = cfg.SoulPath
	data["api_addr"] = cfg.MsgGateway.APIAddr
	if strings.TrimSpace(data["api_addr"].(string)) == "" {
		data["api_addr"] = cfg.Server.Addr
	}
	data["telegram_platform"] = cfg.MsgGateway.Platform
	data["telegram_proxy"] = cfg.MsgGateway.Telegram.Proxy
	data["telegram_timeout_seconds"] = cfg.MsgGateway.Telegram.ChatTimeoutSeconds
	timeoutEvents, timeoutEventsErr := gateway.ReadTimeoutEventsSince(homeDir, time.Now().Add(-24*time.Hour))
	data["timeout_recommendations"] = buildTimeoutRecommendations(cfg, timeoutEvents)
	if timeoutEventsErr == nil {
		byLayer := map[string]int{}
		for _, event := range timeoutEvents {
			byLayer[event.Layer]++
		}
		data["timeout_events_24h"] = len(timeoutEvents)
		data["timeout_events_by_layer"] = byLayer
		if last, err := gateway.ReadTimeoutEvent(homeDir); err == nil {
			data["timeout_last_error"] = map[string]interface{}{
				"layer": last.Layer, "config_path": last.ConfigPath,
				"configured_seconds": last.ConfiguredSeconds,
				"updated_at":         last.UpdatedAt.Format(time.RFC3339),
			}
		}
	}

	if sessions := p.agent.Sessions(); sessions != nil {
		infos := sessions.ListInfo()
		recent := make([]map[string]interface{}, 0, minInt(len(infos), 5))
		for i, info := range infos {
			if i >= 5 {
				break
			}
			recent = append(recent, map[string]interface{}{
				"id":            info.ID,
				"title":         info.Title,
				"message_count": info.MessageCount,
				"updated_at":    info.UpdatedAt.Format(time.RFC3339),
			})
		}
		data["sessions_total"] = sessions.Count()
		data["sessions_recent"] = recent
	}

	if mem := p.agent.Memory(); mem != nil {
		stats := mem.Stats()
		data["memory_short"] = stats[memory.TierShort]
		data["memory_medium"] = stats[memory.TierMedium]
		data["memory_long"] = stats[memory.TierLong]
		data["memory_total"] = mem.Count()
	}

	if cronEngine := p.agent.CronEngine(); cronEngine != nil {
		jobs := cronEngine.ListJobs()
		recentJobs := make([]map[string]interface{}, 0, minInt(len(jobs), 5))
		for i, job := range jobs {
			if i >= 5 {
				break
			}
			recentJobs = append(recentJobs, map[string]interface{}{
				"id":        job.ID,
				"status":    job.Status.String(),
				"next_run":  formatTime(job.NextRun),
				"last_run":  formatTime(job.LastRun),
				"schedule":  describeCronSchedule(job),
				"run_count": job.RunCount,
			})
		}
		data["cron_running"] = cronEngine.IsRunning()
		data["cron_jobs_total"] = cronEngine.JobCount()
		data["cron_jobs"] = recentJobs
	}

	if gm := p.agent.MsgGateway(); gm != nil {
		gatewayNames := gm.List()
		data["gateway_manager_running"] = gm.IsRunning()
		data["gateways_registered"] = gatewayNames
		data["gateway_stats"] = gm.AllStats()
		if gw, ok := gm.Get("telegram"); ok {
			data["telegram_registered"] = true
			data["telegram_connected"] = gw.IsRunning()
		} else {
			data["telegram_registered"] = false
			data["telegram_connected"] = false
		}
		if stats, ok := gm.Stats("telegram"); ok {
			data["telegram_messages_sent"] = stats.MessagesSent
			data["telegram_messages_received"] = stats.MessagesReceived
			data["telegram_errors"] = stats.Errors
		} else {
			data["telegram_messages_sent"] = 0
			data["telegram_messages_received"] = 0
			data["telegram_errors"] = 0
		}
		data["telegram_state_source"] = "local_memory"
		data["telegram_state_updated_at"] = time.Now().Format(time.RFC3339)
		data["telegram_state_pid"] = 0
	}
	if sharedState, err := gateway.ReadSharedTelegramState(homeDir); err == nil && sharedState.IsFresh(15*time.Second) {
		data["telegram_registered"] = sharedState.Registered
		data["telegram_connected"] = sharedState.Connected
		data["telegram_messages_sent"] = sharedState.MessagesSent
		data["telegram_messages_received"] = sharedState.MessagesReceived
		data["telegram_errors"] = sharedState.Errors
		data["telegram_state_source"] = "shared_runtime"
		data["telegram_state_updated_at"] = sharedState.UpdatedAt.Format(time.RFC3339)
		data["telegram_state_pid"] = sharedState.PID
	}

	if tools := p.agent.Tools(); tools != nil {
		allTools := tools.List()
		data["tools_total"] = tools.Count()
		data["tools_enabled"] = len(tools.ListEnabled())
		data["tools_builtin_total"] = len(tools.ListByCategory(tool.CatBuiltin))
		data["tools_skill_total"] = len(tools.ListByCategory(tool.CatSkill))
		data["tools_mcp_total"] = len(tools.ListByCategory(tool.CatMCP))
		data["tools_delegate_total"] = len(tools.ListByCategory(tool.CatDelegate))
		data["tools_model_visible_total"] = len(tools.ListModelVisible())
		data["tools_sample"] = sampleToolNames(allTools, 10)
	}
	skills := p.agent.Skills()
	data["skills_loaded"] = len(skills)
	data["skills_names"] = sampleSkillNames(skills, 10)

	if m := p.agent.Metrics(); m != nil {
		snap := m.Snapshot()
		data["metrics"] = snap
		data["total_requests"] = snap.TotalRequests
		data["chat_requests"] = snap.ChatRequests
		data["tool_calls"] = snap.ToolCalls
		data["function_calls"] = snap.FunctionCalls
		data["error_requests"] = snap.ErrorRequests
		data["metrics_uptime"] = snap.Uptime
	}

	return data
}

// buildTimeoutRecommendations exposes safe, copy-pasteable timeout profiles to
// the Dashboard. It intentionally returns configuration paths and numeric
// values only; credentials, prompts, and timeout event contents are excluded.
// The current values are included next to each profile so operators can see
// which settings would change before applying a suggestion.
func buildTimeoutRecommendations(cfg *config.Config, events []gateway.TimeoutEvent) []map[string]interface{} {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	current := map[string]int{
		"msg_gateway.telegram.chat_timeout_seconds": cfg.MsgGateway.Telegram.ChatTimeoutSeconds,
		"agent.timeout_seconds":                     cfg.Agent.TimeoutSeconds,
		"opencli.timeout_seconds":                   cfg.OpenCLI.TimeoutSeconds,
		"tools.computer_use.timeout_seconds":        cfg.Tools.ComputerUse.TimeoutSeconds,
		"tools.computer_use.step_timeout_seconds":   cfg.Tools.ComputerUse.StepTimeoutSeconds,
	}

	profiles := []struct {
		id        string
		label     string
		reason    string
		suggested map[string]int
	}{
		{
			id:     "quick_response",
			label:  "快速响应",
			reason: "适合短请求和低延迟交互，避免单轮工具等待过久。",
			suggested: map[string]int{
				"msg_gateway.telegram.chat_timeout_seconds": 300,
				"agent.timeout_seconds":                     30,
				"opencli.timeout_seconds":                   15,
			},
		},
		{
			id:     "complex_tasks",
			label:  "复杂任务",
			reason: "适合多工具、文件处理和 Computer Use 等需要更长等待的任务。",
			suggested: map[string]int{
				"msg_gateway.telegram.chat_timeout_seconds": 1200,
				"agent.timeout_seconds":                     120,
				"opencli.timeout_seconds":                   60,
				"tools.computer_use.timeout_seconds":        600,
				"tools.computer_use.step_timeout_seconds":   60,
			},
		},
	}

	result := make([]map[string]interface{}, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, map[string]interface{}{
			"id":              profile.id,
			"label":           profile.label,
			"reason":          profile.reason,
			"priority":        timeoutRecommendationPriority(profile.id, events),
			"evidence_events": len(events),
			"current":         current,
			"suggested":       profile.suggested,
		})
	}
	return result
}

func timeoutRecommendationPriority(profileID string, events []gateway.TimeoutEvent) string {
	if len(events) == 0 {
		return "info"
	}
	for _, event := range events {
		if strings.EqualFold(strings.TrimSpace(event.Layer), "Telegram Gateway Chat") {
			if profileID == "complex_tasks" {
				return "recommended"
			}
			return "alternative"
		}
	}
	return "recommended"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func describeCronSchedule(job *cron.Job) string {
	if job == nil {
		return ""
	}
	if text := strings.TrimSpace(job.Metadata["schedule_text"]); text != "" {
		return text
	}
	if job.Schedule == nil {
		return ""
	}
	return cron.DescribeSchedule(job.Schedule)
}

func sampleToolNames(tools []*tool.Tool, limit int) []string {
	if len(tools) == 0 {
		return nil
	}
	names := make([]string, 0, minInt(len(tools), limit))
	for i, t := range tools {
		if i >= limit || t == nil {
			break
		}
		names = append(names, t.Name)
	}
	return names
}

func sampleSkillNames(skills []*tool.SkillInfo, limit int) []string {
	if len(skills) == 0 {
		return nil
	}
	names := make([]string, 0, minInt(len(skills), limit))
	for i, s := range skills {
		if i >= limit || s == nil {
			break
		}
		names = append(names, s.Name)
	}
	return names
}
