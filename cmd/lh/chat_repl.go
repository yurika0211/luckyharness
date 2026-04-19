package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/cron"
	"github.com/yurika0211/luckyharness/internal/dashboard"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/profile"
	"github.com/yurika0211/luckyharness/internal/session"
)

// startREPL 启动交互式 REPL
func startREPL(mgr *config.Manager) error {
	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// 创建 Cron 引擎和 Watcher
	cronEngine := cron.NewEngine()
	watcher := cron.NewWatcher(cronEngine)

	// 创建会话管理器
	home, _ := os.UserHomeDir()
	sessionMgr, err := session.NewManager(filepath.Join(home, ".luckyharness", "sessions"))
	if err != nil {
		return fmt.Errorf("create session manager: %w", err)
	}

	// 创建当前会话
	currentSession := sessionMgr.New()

	// 启动配置热重载
	configWatcher, _ := mgr.WatchConfig(5 * time.Second)
	configWatcher.OnChange(func(oldCfg, newCfg *config.Config) {
		diff := config.DiffConfig(oldCfg, newCfg)
		if diff.HasChanged() {
			fmt.Println("\n📋 配置已更新:")
			fmt.Print(diff.Format())
			fmt.Println("  重启后生效")
		}
	})
	configWatcher.Start()
	defer configWatcher.Stop()

	cfg := mgr.Get()
	fmt.Println("🍀 LuckyHarness Chat v0.11.0")
	fmt.Printf("   Provider: %s | Model: %s\n", cfg.Provider, cfg.Model)
	fmt.Printf("   会话: %s\n", currentSession.ID[:8])
	fmt.Println("   输入 /quit 退出 | /help 查看命令 | /yolo 自动批准工具调用")
	fmt.Println()

	loopCfg := agent.DefaultLoopConfig()
	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("You> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// 处理命令
		if strings.HasPrefix(input, "/") {
			handled, exit := handleCommand(input, a, &loopCfg, cronEngine, watcher, sessionMgr, &currentSession, mgr)
			if exit {
				break
			}
			if handled {
				continue
			}
		}

		// 执行 Agent Loop
		fmt.Print("Lucky> ")
		result, err := a.RunLoop(ctx, input, loopCfg)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			continue
		}

		fmt.Println(result.Response)

		// 保存到会话
		currentSession.AddMessage("user", input)
		currentSession.AddMessage("assistant", result.Response)

		// 显示工具调用信息
		if len(result.ToolCalls) > 0 {
			fmt.Println()
			for _, tc := range result.ToolCalls {
				fmt.Printf("  🔧 %s(%s) → %s (%v)\n", tc.Name, truncate(tc.Arguments, 50), truncate(tc.Result, 80), tc.Duration)
			}
		}

		// 显示循环信息
		if result.Iterations > 1 {
			fmt.Printf("  🔄 %d iterations | %d tokens\n", result.Iterations, result.TokensUsed)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner: %w", err)
	}
	return nil
}

// handleCommand 处理 REPL 命令
func handleCommand(input string, a *agent.Agent, loopCfg *agent.LoopConfig, cronEngine *cron.Engine, watcher *cron.Watcher, sessionMgr *session.Manager, currentSession **session.Session, cfgMgr *config.Manager) (handled bool, exit bool) {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Println("👋 Bye!")
		return true, true

	case "/help":
		fmt.Println("📋 命令列表:")
		fmt.Println("  /quit, /exit       退出")
		fmt.Println("  /help              显示帮助")
		fmt.Println("  /yolo              切换自动批准工具调用")
		fmt.Println("  /model [name]      切换模型 (无参数显示当前)")
		fmt.Println("  /models            列出可用模型")
		fmt.Println("  /soul              显示当前 SOUL")
		fmt.Println("  /tools             列出可用工具 (含权限)")
		fmt.Println("  /skills [dir]      加载 Skill 插件")
		fmt.Println("  /mcp <name> <url>  连接 MCP Server")
		fmt.Println("  /approve <tool>    设置工具自动批准")
		fmt.Println("  /deny <tool>       禁止工具使用")
		fmt.Println("  /remember [x]      保存中期记忆")
		fmt.Println("  /remember-long [x] 保存长期记忆")
		fmt.Println("  /recall [x]       搜索记忆")
		fmt.Println("  /memstats          记忆统计")
		fmt.Println("  /memdecay          执行记忆衰减")
		fmt.Println("  /promote [id]      提升记忆层级")
		fmt.Println("  /sessions          列出所有会话")
		fmt.Println("  /session new       创建新会话")
		fmt.Println("  /session switch ID 切换会话")
		fmt.Println("  /session search KW 搜索会话")
		fmt.Println("  /session save      保存当前会话")
		fmt.Println("  /session delete ID 删除会话")
		fmt.Println("  /reload            重新加载配置")
		fmt.Println("  /cron add <id> <schedule> <cmd>  添加定时任务")
		fmt.Println("  /cron list         列出定时任务")
		fmt.Println("  /cron remove <id>  移除定时任务")
		fmt.Println("  /cron pause <id>  暂停定时任务")
		fmt.Println("  /cron resume <id> 恢复定时任务")
		fmt.Println("  /cron start       启动调度引擎")
		fmt.Println("  /cron stop         停止调度引擎")
		fmt.Println("  /watch add <id> <pattern> <interval>  添加监控模式")
		fmt.Println("  /watch list        列出监控模式")
		fmt.Println("  /watch remove <id> 移除监控模式")
		fmt.Println("  /watch start       启动监控")
		fmt.Println("  /watch stop        停止监控")
		fmt.Println("  /profile list      列出 Profile")
		fmt.Println("  /profile switch X  切换 Profile")
		fmt.Println("  /dashboard start   启动 Web Dashboard")
		fmt.Println("  /clear             清屏")
		return true, false

	case "/yolo":
		loopCfg.AutoApprove = !loopCfg.AutoApprove
		if loopCfg.AutoApprove {
			fmt.Println("🚀 YOLO mode ON — 工具调用自动批准")
		} else {
			fmt.Println("🔒 YOLO mode OFF — 工具调用需确认")
		}
		return true, false

	case "/model":
		if arg == "" {
			fmt.Printf("当前模型: %s\n", a.Provider().Name())
		} else {
			if err := a.SwitchModel(arg); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("✅ 已切换到模型: %s\n", arg)
			}
		}
		return true, false

	case "/models":
		models := a.Catalog().List()
		if len(models) == 0 {
			fmt.Println("📋 模型目录为空")
		} else {
			fmt.Println("📋 可用模型:")
			currentProvider := ""
			for _, m := range models {
				if m.Provider != currentProvider {
					currentProvider = m.Provider
					fmt.Printf("\n  [%s]\n", currentProvider)
				}
				costInfo := ""
				if m.CostPer1kIn > 0 {
					costInfo = fmt.Sprintf(" ($%.4f/$%.4f per 1k)", m.CostPer1kIn, m.CostPer1kOut)
				} else {
					costInfo = " (免费/本地)"
				}
				fmt.Printf("    %-40s %s%s\n", m.ID, m.DisplayName, costInfo)
			}
		}
		return true, false

	case "/soul":
		fmt.Println(a.Soul().SystemPrompt())
		return true, false

	case "/tools":
		list := a.Tools().FormatToolList()
		if list == "" {
			fmt.Println("🔧 暂无注册工具")
		} else {
			fmt.Println("🔧 可用工具:")
			fmt.Println(list)
		}
		return true, false

	case "/remember":
		if arg == "" {
			fmt.Println("用法: /remember <content>")
		} else {
			if err := a.Remember(arg, "user"); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("💾 已保存为中期记忆")
			}
		}
		return true, false

	case "/remember-long":
		if arg == "" {
			fmt.Println("用法: /remember-long <content>")
		} else {
			if err := a.RememberLongTerm(arg, "user"); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("🧠 已保存为长期记忆（核心记忆）")
			}
		}
		return true, false

	case "/recall":
		if arg == "" {
			fmt.Println("用法: /recall <query>")
		} else {
			results := a.Recall(arg)
			if len(results) == 0 {
				fmt.Println("🔍 未找到相关记忆")
			} else {
				fmt.Printf("🔍 找到 %d 条记忆:\n", len(results))
				for _, e := range results {
					tierLabel := tierEmoji(e.Tier)
					fmt.Printf("  %s [%s] %s (重要度:%.1f 访问:%d)\n",
						tierLabel, e.Tier.String(), e.Content, e.Importance, e.AccessCount)
				}
			}
		}
		return true, false

	case "/memstats":
		stats := a.MemoryStats()
		fmt.Println("📊 记忆统计:")
		fmt.Printf("  🟡 短期 (会话): %d 条\n", stats[memory.TierShort])
		fmt.Printf("  🔵 中期 (日常): %d 条\n", stats[memory.TierMedium])
		fmt.Printf("  🟢 长期 (核心): %d 条\n", stats[memory.TierLong])
		total := stats[memory.TierShort] + stats[memory.TierMedium] + stats[memory.TierLong]
		fmt.Printf("  📦 总计: %d 条\n", total)
		return true, false

	case "/memdecay":
		deleted := a.DecayMemory(0.05)
		fmt.Printf("🗑️ 已衰减 %d 条低权重记忆\n", deleted)
		return true, false

	case "/promote":
		if arg == "" {
			fmt.Println("用法: /promote <memory-id>")
		} else {
			if err := a.PromoteMemory(arg); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("⬆️ 记忆层级已提升")
			}
		}
		return true, false

	case "/clear":
		fmt.Print("\033[2J\033[H")
		return true, false

	case "/sessions":
		infos := sessionMgr.ListInfo()
		if len(infos) == 0 {
			fmt.Println("📋 暂无会话")
		} else {
			fmt.Println("📋 会话列表:")
			for _, info := range infos {
				active := ""
				if info.ID == (*currentSession).ID {
					active = " ← 当前"
				}
				title := info.Title
				if title == "" {
					title = "(无标题)"
				}
				fmt.Printf("  %s | %s | %d 条消息 | %s%s\n",
					info.ID[:8], title, info.MessageCount,
					info.UpdatedAt.Format("2006-01-02 15:04"), active)
			}
		}
		return true, false

	case "/session":
		return handleSessionCommand(arg, sessionMgr, currentSession), false

	case "/reload":
		if err := cfgMgr.Reload(); err != nil {
			fmt.Printf("❌ 重载配置失败: %v\n", err)
		} else {
			cfg := cfgMgr.Get()
			fmt.Printf("✅ 配置已重载 | Provider: %s | Model: %s\n", cfg.Provider, cfg.Model)
		}
		return true, false

	case "/skills":
		if arg == "" {
			fmt.Println("用法: /skills <directory>")
		} else {
			count, err := a.LoadSkills(arg)
			if err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("✅ 已加载 %d 个 Skill 插件\n", count)
			}
		}
		return true, false

	case "/mcp":
		parts := strings.Fields(arg)
		if len(parts) < 2 {
			fmt.Println("用法: /mcp <name> <url> [api_key]")
		} else {
			apiKey := ""
			if len(parts) > 2 {
				apiKey = parts[2]
			}
			a.ConnectMCPServer(parts[0], parts[1], apiKey)
			fmt.Printf("✅ 已连接 MCP Server: %s (%s)\n", parts[0], parts[1])
		}
		return true, false

	case "/approve":
		if arg == "" {
			fmt.Println("用法: /approve <tool_name>")
		} else {
			if err := a.Tools().SetPermissionOverride(arg, 0); err != nil { // PermAuto = 0
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("✅ 工具 %s 已设为自动批准\n", arg)
			}
		}
		return true, false

	case "/deny":
		if arg == "" {
			fmt.Println("用法: /deny <tool_name>")
		} else {
			if err := a.Tools().SetPermissionOverride(arg, 2); err != nil { // PermDeny = 2
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("🔴 工具 %s 已禁止使用\n", arg)
			}
		}
		return true, false

	case "/cron":
		return handleCronCommand(arg, cronEngine), false

	case "/watch":
		return handleWatchCommand(arg, watcher), false

	case "/profile":
		return handleProfileCommand(arg), false

	case "/dashboard":
		return handleDashboardCommand(arg), false

	default:
		fmt.Printf("未知命令: %s (输入 /help 查看帮助)\n", cmd)
		return true, false
	}
}

func tierEmoji(t memory.Tier) string {
	switch t {
	case memory.TierShort:
		return "🟡"
	case memory.TierMedium:
		return "🔵"
	case memory.TierLong:
		return "🟢"
	default:
		return "⚪"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// handleCronCommand 处理 /cron 命令
func handleCronCommand(arg string, engine *cron.Engine) bool {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		fmt.Println("用法: /cron <add|list|remove|pause|resume|start|stop> [args]")
		return true
	}

	subCmd := parts[0]
	switch subCmd {
	case "add":
		if len(parts) < 4 {
			fmt.Println("用法: /cron add <id> <schedule> <command>")
			fmt.Println("  schedule: 每天9点 | 每小时 | 每30分钟 | 每周一9点 | 工作日9点 | 0 9 * * *")
			return true
		}
		id := parts[1]
		scheduleStr := parts[2]
		command := strings.Join(parts[3:], " ")

		// 尝试自然语言解析，失败则尝试 cron 表达式
		var schedule cron.Schedule
		var err error
		schedule, err = cron.ParseNaturalLanguage(scheduleStr)
		if err != nil {
			schedule, err = cron.ParseCronExpr(scheduleStr)
			if err != nil {
				fmt.Printf("❌ 无法解析调度表达式: %v\n", err)
				return true
			}
		}

		// 创建简单任务（打印命令）
		task := func() error {
			fmt.Printf("\n⏰ [cron:%s] %s\n", id, command)
			return nil
		}

		if err := engine.AddJob(id, "Cron: "+id, command, schedule, task); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ 定时任务已添加: %s (%s)\n", id, schedule)
		}

	case "list":
		jobs := engine.ListJobs()
		if len(jobs) == 0 {
			fmt.Println("📋 暂无定时任务")
		} else {
			fmt.Println("📋 定时任务:")
			for _, j := range jobs {
				statusEmoji := "⏸️"
				if j.Status == cron.StatusRunning {
					statusEmoji = "▶️"
				} else if j.Status == cron.StatusFailed {
					statusEmoji = "❌"
				} else if j.Status == cron.StatusPaused {
					statusEmoji = "⏸️"
				} else {
					statusEmoji = "⏳"
				}
				nextRun := "N/A"
				if !j.NextRun.IsZero() {
					nextRun = j.NextRun.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("  %s %s | %s | 下次: %s | 执行: %d | %s\n",
					statusEmoji, j.ID, j.Schedule, nextRun, j.RunCount, j.Description)
			}
		}

	case "remove":
		if len(parts) < 2 {
			fmt.Println("用法: /cron remove <id>")
			return true
		}
		if err := engine.RemoveJob(parts[1]); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ 定时任务已移除: %s\n", parts[1])
		}

	case "pause":
		if len(parts) < 2 {
			fmt.Println("用法: /cron pause <id>")
			return true
		}
		if err := engine.PauseJob(parts[1]); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("⏸️ 定时任务已暂停: %s\n", parts[1])
		}

	case "resume":
		if len(parts) < 2 {
			fmt.Println("用法: /cron resume <id>")
			return true
		}
		if err := engine.ResumeJob(parts[1]); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("▶️ 定时任务已恢复: %s\n", parts[1])
		}

	case "start":
		engine.Start()
		fmt.Println("▶️ 调度引擎已启动")

	case "stop":
		engine.Stop()
		fmt.Println("⏹️ 调度引擎已停止")

	default:
		fmt.Printf("未知 cron 子命令: %s\n", subCmd)
		fmt.Println("用法: /cron <add|list|remove|pause|resume|start|stop> [args]")
	}
	return true
}

// handleWatchCommand 处理 /watch 命令
func handleWatchCommand(arg string, watcher *cron.Watcher) bool {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		fmt.Println("用法: /watch <add|list|remove|start|stop> [args]")
		return true
	}

	subCmd := parts[0]
	switch subCmd {
	case "add":
		if len(parts) < 4 {
			fmt.Println("用法: /watch add <id> <pattern> <interval>")
			fmt.Println("  interval: 30s | 1m | 5m | 1h")
			return true
		}
		id := parts[1]
		pattern := parts[2]
		interval, err := time.ParseDuration(parts[3])
		if err != nil {
			fmt.Printf("❌ 无法解析间隔: %v\n", err)
			return true
		}

		if err := watcher.AddPattern(id, "Watch: "+id, pattern, pattern, interval, nil); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ 监控模式已添加: %s (%s, 每%s检查)\n", id, pattern, interval)
		}

	case "list":
		patterns := watcher.ListPatterns()
		if len(patterns) == 0 {
			fmt.Println("📋 暂无监控模式")
		} else {
			fmt.Println("📋 监控模式:")
			for _, p := range patterns {
				lastCheck := "N/A"
				if !p.LastCheck.IsZero() {
					lastCheck = p.LastCheck.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("  🔍 %s | %s | 间隔: %s | 上次检查: %s | %s\n",
					p.ID, p.Pattern, p.Interval, lastCheck, p.LastResult)
			}
		}

	case "remove":
		if len(parts) < 2 {
			fmt.Println("用法: /watch remove <id>")
			return true
		}
		if err := watcher.RemovePattern(parts[1]); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ 监控模式已移除: %s\n", parts[1])
		}

	case "start":
		watcher.Start()
		fmt.Println("▶️ 监控已启动")

	case "stop":
		watcher.Stop()
		fmt.Println("⏹️ 监控已停止")

	default:
		fmt.Printf("未知 watch 子命令: %s\n", subCmd)
		fmt.Println("用法: /watch <add|list|remove|start|stop> [args]")
	}
	return true
}

// handleProfileCommand 处理 /profile 命令
func handleProfileCommand(arg string) bool {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		fmt.Println("用法: /profile <list|switch> [args]")
		return true
	}

	home, _ := os.UserHomeDir()
	mgr, err := profile.NewManager(filepath.Join(home, ".luckyharness"))
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return true
	}

	switch parts[0] {
	case "list":
		infos := mgr.ListWithInfo()
		if len(infos) == 0 {
			fmt.Println("📋 暂无 Profile")
		} else {
			fmt.Println("📋 Profiles:")
			for _, info := range infos {
				active := ""
				if info.Active {
					active = " ← active"
				}
				fmt.Printf("  %-15s %-10s %-20s%s\n", info.Name, info.Provider, info.Model, active)
			}
		}

	case "switch":
		if len(parts) < 2 {
			fmt.Println("用法: /profile switch <name>")
			return true
		}
		if err := mgr.Switch(parts[1]); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ 已切换到 Profile: %s (下次启动生效)\n", parts[1])
		}

	default:
		fmt.Printf("未知 profile 子命令: %s\n", parts[0])
		fmt.Println("用法: /profile <list|switch> [args]")
	}
	return true
}

// handleDashboardCommand 处理 /dashboard 命令
func handleDashboardCommand(arg string) bool {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		fmt.Println("用法: /dashboard start [addr]")
		return true
	}

	switch parts[0] {
	case "start":
		addr := ":8765"
		if len(parts) > 1 {
			addr = parts[1]
		}
		cfg := dashboard.Config{Addr: addr}
		d := dashboard.New(cfg)
		if err := d.Start(); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("🌐 Dashboard 已启动: http://localhost%s\n", addr)
		}

	default:
		fmt.Printf("未知 dashboard 子命令: %s\n", parts[0])
	}
	return true
}

// handleSessionCommand 处理 /session 命令
func handleSessionCommand(arg string, mgr *session.Manager, currentSession **session.Session) bool {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		fmt.Println("用法: /session <new|switch|search|save|delete> [args]")
		return true
	}

	switch parts[0] {
	case "new":
		s := mgr.New()
		*currentSession = s
		fmt.Printf("✅ 新会话已创建: %s\n", s.ID[:8])

	case "switch":
		if len(parts) < 2 {
			fmt.Println("用法: /session switch <id-prefix>")
			return true
		}
		// 支持前缀匹配
		idPrefix := parts[1]
		sessions := mgr.List()
		var found *session.Session
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, idPrefix) {
				found = s
				break
			}
		}
		if found == nil {
			fmt.Printf("❌ 未找到会话: %s\n", idPrefix)
		} else {
			*currentSession = found
			fmt.Printf("✅ 已切换到会话: %s (%d 条消息)\n", found.ID[:8], found.MessageCount())
		}

	case "search":
		if len(parts) < 2 {
			fmt.Println("用法: /session search <keyword>")
			return true
		}
		query := strings.Join(parts[1:], " ")
		results := mgr.Search(query)
		if len(results) == 0 {
			fmt.Println("🔍 未找到匹配的会话")
		} else {
			fmt.Printf("🔍 找到 %d 个会话:\n", len(results))
			for _, info := range results {
				title := info.Title
				if title == "" {
					title = "(无标题)"
				}
				fmt.Printf("  %s | %s | %d 条消息\n",
					info.ID[:8], title, info.MessageCount)
			}
		}

	case "save":
		if err := (*currentSession).Save(); err != nil {
			fmt.Printf("❌ 保存失败: %v\n", err)
		} else {
			fmt.Printf("✅ 会话已保存: %s\n", (*currentSession).ID[:8])
		}

	case "delete":
		if len(parts) < 2 {
			fmt.Println("用法: /session delete <id-prefix>")
			return true
		}
		idPrefix := parts[1]
		sessions := mgr.List()
		var targetID string
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, idPrefix) {
				targetID = s.ID
				break
			}
		}
		if targetID == "" {
			fmt.Printf("❌ 未找到会话: %s\n", idPrefix)
		} else if targetID == (*currentSession).ID {
			fmt.Println("❌ 不能删除当前活跃会话")
		} else {
			if err := mgr.Delete(targetID); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("✅ 会话已删除: %s\n", targetID[:8])
			}
		}

	default:
		fmt.Printf("未知 session 子命令: %s\n", parts[0])
		fmt.Println("用法: /session <new|switch|search|save|delete> [args]")
	}
	return true
}