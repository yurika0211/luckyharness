package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/memory"
)

// startREPL 启动交互式 REPL
func startREPL(mgr *config.Manager) error {
	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	cfg := mgr.Get()
	fmt.Println("🍀 LuckyHarness Chat v0.4.0")
	fmt.Printf("   Provider: %s | Model: %s\n", cfg.Provider, cfg.Model)
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
			handled, exit := handleCommand(input, a, &loopCfg)
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
func handleCommand(input string, a *agent.Agent, loopCfg *agent.LoopConfig) (handled bool, exit bool) {
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
		fmt.Println("  /tools             列出可用工具")
		fmt.Println("  /remember [x]      保存中期记忆")
		fmt.Println("  /remember-long [x] 保存长期记忆")
		fmt.Println("  /recall [x]       搜索记忆")
		fmt.Println("  /memstats          记忆统计")
		fmt.Println("  /memdecay          执行记忆衰减")
		fmt.Println("  /promote [id]      提升记忆层级")
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
		tools := a.Tools().List()
		if len(tools) == 0 {
			fmt.Println("🔧 暂无注册工具")
		} else {
			fmt.Println("🔧 可用工具:")
			for _, t := range tools {
				fmt.Printf("  - %s: %s\n", t.Name, t.Description)
			}
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