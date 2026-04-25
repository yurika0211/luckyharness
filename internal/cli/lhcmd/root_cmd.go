package lhcmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/logger"
)

type commandStartTimeKey struct{}

var commandLoggerInitOnce sync.Once

func initCommandLogger() {
	commandLoggerInitOnce.Do(func() {
		logCfg := logger.DefaultConfig()

		// 尝试从配置读取日志级别/格式；失败时回退默认值。
		if mgr, err := config.NewManager(); err == nil {
			if err := mgr.Load(); err == nil {
				cfg := mgr.Get()
				if cfg.Server.LogLevel != "" {
					logCfg.Level = cfg.Server.LogLevel
				}
				if cfg.Server.LogFormat != "" {
					logCfg.Format = cfg.Server.LogFormat
				}
			}
		}

		logger.InitLogger(logCfg)
	})
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "lh",
		Short: "🍀 LuckyHarness — Go 版自主 AI Agent 框架",
		Long:  "LuckyHarness 是一个用 Go 重写的 AI Agent 框架，参考 Hermes Agent 架构迭代开发。",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initCommandLogger()

			startAt := time.Now()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			cmd.SetContext(context.WithValue(ctx, commandStartTimeKey{}, startAt))

			logger.Info("command started",
				"command", cmd.CommandPath(),
				"args_count", len(args),
			)
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			var duration time.Duration
			if ctx := cmd.Context(); ctx != nil {
				if startAt, ok := ctx.Value(commandStartTimeKey{}).(time.Time); ok && !startAt.IsZero() {
					duration = time.Since(startAt)
				}
			}

			logger.Info("command completed",
				"command", cmd.CommandPath(),
				"duration_ms", duration.Milliseconds(),
			)
		},
	}

	// init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "初始化 LuckyHarness 主目录",
		RunE:  runInit,
	}

	// chat
	chatCmd := &cobra.Command{
		Use:   "chat [message]",
		Short: "开始对话 (无参数进入交互模式)",
		RunE:  runChat,
	}
	chatCmd.Flags().StringVarP(&soulFile, "soul", "s", "", "SOUL.md 文件路径")
	chatCmd.Flags().StringVarP(&provider_, "provider", "p", "", "LLM 提供商")
	chatCmd.Flags().StringVarP(&model_, "model", "m", "", "模型名称")
	chatCmd.Flags().BoolVar(&yolo, "yolo", false, "自动批准所有工具调用")

	// config
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "管理配置",
	}
	configGetCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "获取配置项",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}
	configSetCmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "设置配置项",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
	configListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有配置",
		RunE:  runConfigList,
	}
	configCmd.AddCommand(configGetCmd, configSetCmd, configListCmd)

	// soul
	soulCmd := &cobra.Command{
		Use:   "soul",
		Short: "管理 SOUL",
	}
	soulShowCmd := &cobra.Command{
		Use:   "show",
		Short: "显示当前 SOUL",
		RunE:  runSoulShow,
	}
	soulListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有 SOUL 模板",
		RunE:  runSoulList,
	}
	soulListCmd.Flags().StringP("language", "l", "", "按语言过滤 (zh/en/ja/ko)")
	soulSwitchCmd := &cobra.Command{
		Use:   "switch <template-id>",
		Short: "切换到指定 SOUL 模板",
		Args:  cobra.ExactArgs(1),
		RunE:  runSoulSwitch,
	}
	soulCmd.AddCommand(soulShowCmd, soulListCmd, soulSwitchCmd)

	// version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🍀 LuckyHarness v0.38.2")
		},
	}

	// models
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "列出可用模型",
		RunE:  runModels,
	}

	// ===== v0.9.0: Profile 命令 =====
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "管理多实例 Profile",
	}
	profileListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有 Profile",
		RunE:  runProfileList,
	}
	profileShowCmd := &cobra.Command{
		Use:   "show [name]",
		Short: "显示 Profile 详情",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runProfileShow,
	}
	profileCreateCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "创建新 Profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileCreate,
	}
	profileCreateCmd.Flags().StringP("desc", "d", "", "Profile 描述")
	profileDeleteCmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "删除 Profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileDelete,
	}
	profileSwitchCmd := &cobra.Command{
		Use:   "switch [name]",
		Short: "切换活跃 Profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileSwitch,
	}
	profileEnvCmd := &cobra.Command{
		Use:   "env [name]",
		Short: "管理 Profile 环境变量",
	}
	profileEnvSetCmd := &cobra.Command{
		Use:   "set [profile] [key] [value]",
		Short: "设置环境变量",
		Args:  cobra.ExactArgs(3),
		RunE:  runProfileEnvSet,
	}
	profileEnvUnsetCmd := &cobra.Command{
		Use:   "unset [profile] [key]",
		Short: "删除环境变量",
		Args:  cobra.ExactArgs(2),
		RunE:  runProfileEnvUnset,
	}
	profileEnvCmd.AddCommand(profileEnvSetCmd, profileEnvUnsetCmd)
	profileCmd.AddCommand(profileListCmd, profileShowCmd, profileCreateCmd, profileDeleteCmd, profileSwitchCmd, profileEnvCmd)

	// ===== v0.9.0: Backup 命令 =====
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "备份与恢复",
	}
	backupCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "创建备份",
		RunE:  runBackupCreate,
	}
	backupCreateCmd.Flags().StringP("output", "o", "", "输出路径")
	backupRestoreCmd := &cobra.Command{
		Use:   "restore [path]",
		Short: "从备份恢复",
		Args:  cobra.ExactArgs(1),
		RunE:  runBackupRestore,
	}
	backupListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出备份",
		RunE:  runBackupList,
	}
	backupCmd.AddCommand(backupCreateCmd, backupRestoreCmd, backupListCmd)

	// ===== v0.9.0: Dashboard 命令 =====
	dashboardCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Web Dashboard",
	}
	dashboardStartCmd := &cobra.Command{
		Use:   "start",
		Short: "启动 Dashboard",
		RunE:  runDashboardStart,
	}
	dashboardStartCmd.Flags().StringP("addr", "a", ":8765", "监听地址")
	dashboardCmd.AddCommand(dashboardStartCmd)

	// ===== v0.9.0: Debug 命令 =====
	debugCmd := &cobra.Command{
		Use:   "debug",
		Short: "调试工具",
	}
	debugShareCmd := &cobra.Command{
		Use:   "share",
		Short: "导出调试信息",
		RunE:  runDebugShare,
	}
	debugShareCmd.Flags().Bool("no-env", false, "不收集环境变量")
	debugShareCmd.Flags().Bool("no-config", false, "不收集配置")
	debugShareCmd.Flags().Bool("no-logs", false, "不收集日志")
	debugShareCmd.Flags().StringP("output", "o", "", "输出路径")
	debugCmd.AddCommand(debugShareCmd)

	// ===== v0.10.0: Gateway 命令 =====
	gatewayCmd := &cobra.Command{
		Use:   "gateway",
		Short: "工具网关管理",
	}
	gatewayInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "显示网关状态",
		RunE:  runGatewayInfo,
	}
	gatewayRouteCmd := &cobra.Command{
		Use:   "route",
		Short: "管理路由规则",
	}
	gatewayRouteListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出路由规则",
		RunE:  runGatewayRouteList,
	}
	gatewayRouteAddCmd := &cobra.Command{
		Use:   "add [name] [pattern] [target] [priority]",
		Short: "添加路由规则",
		Args:  cobra.ExactArgs(4),
		RunE:  runGatewayRouteAdd,
	}
	gatewayRouteRemoveCmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "移除路由规则",
		Args:  cobra.ExactArgs(1),
		RunE:  runGatewayRouteRemove,
	}
	gatewayAliasCmd := &cobra.Command{
		Use:   "alias",
		Short: "管理工具别名",
	}
	gatewayAliasListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出别名",
		RunE:  runGatewayAliasList,
	}
	gatewayAliasAddCmd := &cobra.Command{
		Use:   "add [alias] [target]",
		Short: "添加别名",
		Args:  cobra.ExactArgs(2),
		RunE:  runGatewayAliasAdd,
	}
	gatewayAliasRemoveCmd := &cobra.Command{
		Use:   "remove [alias]",
		Short: "移除别名",
		Args:  cobra.ExactArgs(1),
		RunE:  runGatewayAliasRemove,
	}
	gatewayAliasCmd.AddCommand(gatewayAliasListCmd, gatewayAliasAddCmd, gatewayAliasRemoveCmd)
	gatewayRouteCmd.AddCommand(gatewayRouteListCmd, gatewayRouteAddCmd, gatewayRouteRemoveCmd)
	gatewayCmd.AddCommand(gatewayInfoCmd, gatewayRouteCmd, gatewayAliasCmd)

	// ===== v0.6.0: Messaging Gateway 命令 =====
	msgGatewayCmd := &cobra.Command{
		Use:   "msg-gateway",
		Short: "消息平台网关管理 (Telegram, Discord, etc.)",
	}
	msgGatewayStartCmd := &cobra.Command{
		Use:   "start [--platform telegram --token TOKEN]",
		Short: "启动消息网关",
		RunE:  runMsgGatewayStart,
	}
	msgGatewayStartCmd.Flags().String("platform", "", "平台名称 (telegram, onebot)")
	msgGatewayStartCmd.Flags().String("token", "", "Bot token (Telegram)")
	msgGatewayStartCmd.Flags().String("onebot-api", "", "OneBot HTTP API 地址 (如 http://127.0.0.1:3000)")
	msgGatewayStartCmd.Flags().String("onebot-ws", "", "OneBot WebSocket 事件地址 (如 ws://127.0.0.1:3001)")
	msgGatewayStartCmd.Flags().String("onebot-token", "", "OneBot Access Token")
	msgGatewayStartCmd.Flags().String("onebot-bot-id", "", "OneBot Bot QQ ID")
	msgGatewayStartCmd.Flags().Bool("onebot-typing", true, "OneBot 显示正在输入")
	msgGatewayStartCmd.Flags().Bool("onebot-like", true, "OneBot 收到消息自动点赞")
	msgGatewayStartCmd.Flags().Int("onebot-like-times", 1, "OneBot 点赞次数 (1-10)")
	msgGatewayStartCmd.Flags().Bool("all", false, "启动所有已配置的网关")
	msgGatewayStartCmd.Flags().String("api-addr", "127.0.0.1:9090", "HTTP API 监听地址")
	msgGatewayStopCmd := &cobra.Command{
		Use:   "stop [platform]",
		Short: "停止消息网关",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMsgGatewayStop,
	}
	msgGatewayStopCmd.Flags().String("api-addr", "", "消息网关 API 地址（默认读取 msg_gateway.api_addr）")
	msgGatewayStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "查看消息网关状态",
		RunE:  runMsgGatewayStatus,
	}
	msgGatewayStatusCmd.Flags().String("api-addr", "", "消息网关 API 地址（默认读取 msg_gateway.api_addr）")
	msgGatewayCmd.AddCommand(msgGatewayStartCmd, msgGatewayStopCmd, msgGatewayStatusCmd)

	// ===== v0.10.0: Subscription 命令 =====
	subCmd := &cobra.Command{
		Use:   "sub",
		Short: "订阅管理",
	}
	subListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有订阅",
		RunE:  runSubList,
	}
	subInfoCmd := &cobra.Command{
		Use:   "info [user_id]",
		Short: "查看用户订阅详情",
		Args:  cobra.ExactArgs(1),
		RunE:  runSubInfo,
	}
	subSubscribeCmd := &cobra.Command{
		Use:   "subscribe [user_id] [tier] [duration]",
		Short: "订阅 (tier: free/basic/pro/enterprise, duration: e.g. 30d)",
		Args:  cobra.ExactArgs(3),
		RunE:  runSubSubscribe,
	}
	subUnsubscribeCmd := &cobra.Command{
		Use:   "unsubscribe [user_id]",
		Short: "取消订阅",
		Args:  cobra.ExactArgs(1),
		RunE:  runSubUnsubscribe,
	}
	subCmd.AddCommand(subListCmd, subInfoCmd, subSubscribeCmd, subUnsubscribeCmd)

	// ===== v0.10.0: Usage 命令 =====
	usageCmd := &cobra.Command{
		Use:   "usage",
		Short: "工具使用统计",
	}
	usageStatsCmd := &cobra.Command{
		Use:   "stats [user_id]",
		Short: "查看用户使用统计",
		Args:  cobra.ExactArgs(1),
		RunE:  runUsageStats,
	}
	usageQuotaCmd := &cobra.Command{
		Use:   "quota",
		Short: "管理配额",
	}
	usageQuotaSetCmd := &cobra.Command{
		Use:   "set [user_id] [tool_name] [window] [limit]",
		Short: "设置配额 (window: hourly/daily/monthly)",
		Args:  cobra.ExactArgs(4),
		RunE:  runUsageQuotaSet,
	}
	usageQuotaListCmd := &cobra.Command{
		Use:   "list [user_id]",
		Short: "列出用户配额",
		Args:  cobra.ExactArgs(1),
		RunE:  runUsageQuotaList,
	}
	usageQuotaRemoveCmd := &cobra.Command{
		Use:   "remove [user_id] [tool_name]",
		Short: "移除配额",
		Args:  cobra.ExactArgs(2),
		RunE:  runUsageQuotaRemove,
	}
	usageQuotaCmd.AddCommand(usageQuotaSetCmd, usageQuotaListCmd, usageQuotaRemoveCmd)
	usageCmd.AddCommand(usageStatsCmd, usageQuotaCmd)

	// ===== v0.13.0: Serve 命令 =====
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 API Server",
		Long:  "启动 LuckyHarness HTTP API Server，暴露 RESTful API 供外部调用。\n\n端点:\n  POST /api/v1/chat       — 流式聊天 (SSE)\n  POST /api/v1/chat/sync  — 同步聊天\n  GET  /api/v1/sessions   — 会话列表\n  GET  /api/v1/memory     — 记忆统计\n  POST /api/v1/memory     — 保存记忆\n  GET  /api/v1/memory/recall?q= — 搜索记忆\n  GET  /api/v1/tools      — 工具列表\n  GET  /api/v1/stats      — 服务器统计\n  GET  /api/v1/health     — 健康检查",
		RunE:  runServe,
	}
	serveCmd.Flags().StringP("addr", "a", "127.0.0.1:9090", "监听地址")
	serveCmd.Flags().StringSlice("api-keys", nil, "API Key 白名单 (逗号分隔，空=不鉴权)")
	serveCmd.Flags().Bool("no-cors", false, "禁用 CORS")
	serveCmd.Flags().Int("rate-limit", 60, "每分钟请求限制")
	serveCmd.Flags().String("metrics-addr", "", "Prometheus metrics 独立端口 (空=复用主端口)")
	serveCmd.Flags().String("log-level", "info", "日志级别: debug, info, warn, error")
	serveCmd.Flags().String("log-format", "text", "日志格式: json, text")

	// ===== v0.18.0: WebSocket 命令 =====
	wsCmd := &cobra.Command{
		Use:   "ws",
		Short: "WebSocket 管理",
	}
	wsStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "查看 WebSocket 连接统计",
		RunE:  runWSStats,
	}
	wsStatsCmd.Flags().String("addr", "http://localhost:9090", "API Server 地址")
	wsCmd.AddCommand(wsStatsCmd)

	// ===== v0.22.0: Agent 协作命令 =====
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent 协作管理",
	}
	agentListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出注册的 Agent",
		RunE:  runAgentList,
	}
	agentDelegateCmd := &cobra.Command{
		Use:   "delegate <mode> <input> <agent_ids...>",
		Short: "创建协作任务",
		Long:  "创建协作任务。mode: pipeline/parallel/debate\n示例: lh agent delegate parallel \"分析这段代码\" agent-1 agent-2",
		Args:  cobra.MinimumNArgs(3),
		RunE:  runAgentDelegate,
	}
	agentTaskCmd := &cobra.Command{
		Use:   "task <task_id>",
		Short: "查看任务状态",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentTask,
	}
	agentTasksCmd := &cobra.Command{
		Use:   "tasks",
		Short: "列出所有任务",
		RunE:  runAgentTasks,
	}
	agentCancelCmd := &cobra.Command{
		Use:   "cancel <task_id>",
		Short: "取消任务",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentCancel,
	}
	agentCmd.AddCommand(agentListCmd, agentDelegateCmd, agentTaskCmd, agentTasksCmd, agentCancelCmd)

	// ===== v0.17.0: Metrics 命令 =====
	metricsCmd := &cobra.Command{
		Use:   "metrics",
		Short: "查看运行指标",
		Long:  "查看 LuckyHarness 运行指标，包括请求计数、Provider 调用统计、会话数等。\n\n需要 API Server 正在运行。",
		RunE:  runMetrics,
	}

	// ===== v0.14.0: RAG 命令 =====
	ragCmd := &cobra.Command{
		Use:   "rag",
		Short: "RAG 知识库管理",
	}
	ragIndexCmd := &cobra.Command{
		Use:   "index <path>",
		Short: "索引文件或目录到知识库",
		Args:  cobra.ExactArgs(1),
		RunE:  runRAGIndex,
	}
	ragSearchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "搜索知识库",
		Args:  cobra.ExactArgs(1),
		RunE:  runRAGSearch,
	}
	ragStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "知识库统计",
		RunE:  runRAGStats,
	}
	ragCmd.AddCommand(ragIndexCmd, ragSearchCmd, ragStatsCmd)

	// ===== v0.23.0: 流式 RAG 命令 =====
	ragWatchCmd := &cobra.Command{
		Use:   "watch <dir>",
		Short: "添加监控目录",
		Args:  cobra.ExactArgs(1),
		RunE:  runRAGWatch,
	}
	ragUnwatchCmd := &cobra.Command{
		Use:   "unwatch <dir>",
		Short: "移除监控目录",
		Args:  cobra.ExactArgs(1),
		RunE:  runRAGUnwatch,
	}
	ragScanCmd := &cobra.Command{
		Use:   "scan",
		Short: "扫描变更",
		RunE:  runRAGScan,
	}
	ragStartCmd := &cobra.Command{
		Use:   "start",
		Short: "启动后台索引",
		RunE:  runRAGStart,
	}
	ragStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "停止后台索引",
		RunE:  runRAGStop,
	}
	ragStreamStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "流式索引状态",
		RunE:  runRAGStreamStatus,
	}
	ragQueueCmd := &cobra.Command{
		Use:   "queue",
		Short: "查看索引队列",
		RunE:  runRAGQueue,
	}
	ragProcessCmd := &cobra.Command{
		Use:   "process [batch]",
		Short: "处理索引队列",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRAGProcess,
	}
	ragCmd.AddCommand(ragWatchCmd, ragUnwatchCmd, ragScanCmd, ragStartCmd, ragStopCmd,
		ragStreamStatusCmd, ragQueueCmd, ragProcessCmd)

	// ===== v0.15.0: Plugin 命令 =====
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "插件管理",
	}
	pluginInstallCmd := &cobra.Command{
		Use:   "install <path>",
		Short: "安装插件（本地路径）",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginInstall,
	}
	pluginListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出已安装插件",
		RunE:  runPluginList,
	}
	pluginRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "卸载插件",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginRemove,
	}
	pluginUpdateCmd := &cobra.Command{
		Use:   "update <name> <path>",
		Short: "更新插件",
		Args:  cobra.ExactArgs(2),
		RunE:  runPluginUpdate,
	}
	pluginSearchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "搜索插件",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginSearch,
	}
	pluginInfoCmd := &cobra.Command{
		Use:   "info <name>",
		Short: "查看插件详情",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginInfo,
	}
	pluginEnableCmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "启用插件",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginEnable,
	}
	pluginDisableCmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "禁用插件",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginDisable,
	}
	pluginCmd.AddCommand(pluginInstallCmd, pluginListCmd, pluginRemoveCmd,
		pluginUpdateCmd, pluginSearchCmd, pluginInfoCmd,
		pluginEnableCmd, pluginDisableCmd)

	// ---- eval command ----
	evalCmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluation & benchmarking",
	}
	evalRunCmd := &cobra.Command{
		Use:   "run <dir>",
		Short: "Run benchmark against test cases in directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runEvalRun,
	}
	evalListCmd := &cobra.Command{
		Use:   "list <dir>",
		Short: "List test cases in directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runEvalList,
	}
	evalReportCmd := &cobra.Command{
		Use:   "report <result-file>",
		Short: "Display a saved benchmark report",
		Args:  cobra.ExactArgs(1),
		RunE:  runEvalReport,
	}
	evalRunCmd.Flags().StringVarP(&evalFormat, "format", "f", "text", "Report format: text, json, yaml")
	evalRunCmd.Flags().Float64VarP(&evalThreshold, "threshold", "t", 0.7, "Pass threshold (0.0-1.0)")
	evalRunCmd.Flags().StringVarP(&evalOutput, "output", "o", "", "Output file (default: stdout)")
	evalReportCmd.Flags().StringVarP(&evalFormat, "format", "f", "text", "Report format: text, json, yaml")
	evalCmd.AddCommand(evalRunCmd, evalListCmd, evalReportCmd)

	// ---- template command ----
	tmplCmd := &cobra.Command{
		Use:   "template",
		Short: "Prompt template management",
	}
	tmplRenderCmd := &cobra.Command{
		Use:   "render <template-name>",
		Short: "Render a template with variables",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateRender,
	}
	tmplListCmd := &cobra.Command{
		Use:   "list [dir]",
		Short: "List available templates",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTemplateList,
	}
	tmplValidateCmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a template file",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateValidate,
	}
	var tmplVars []string
	tmplRenderCmd.Flags().StringSliceVarP(&tmplVars, "var", "v", nil, "Variables in key=value format")

	tmplCmd.AddCommand(tmplRenderCmd, tmplListCmd, tmplValidateCmd)

	// ---- cost command ----
	costCmd := &cobra.Command{
		Use:   "cost",
		Short: "API cost tracking and budget management",
	}
	costSummaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary",
		RunE:  runCostSummary,
	}
	costDetailCmd := &cobra.Command{
		Use:   "detail",
		Short: "Show recent cost records",
		RunE:  runCostDetail,
	}
	costBudgetCmd := &cobra.Command{
		Use:   "budget",
		Short: "Show budget status",
		RunE:  runCostBudget,
	}
	costSetBudgetCmd := &cobra.Command{
		Use:   "set-budget <period> <limit-usd>",
		Short: "Set a budget (period: daily/weekly/monthly)",
		Args:  cobra.ExactArgs(2),
		RunE:  runCostSetBudget,
	}
	costSummaryCmd.Flags().StringVar(&costProvider, "provider", "", "Filter by provider")
	costSummaryCmd.Flags().StringVar(&costModel, "model", "", "Filter by model")
	costSummaryCmd.Flags().StringVar(&costPeriod, "period", "all", "Time period: today/week/month/all")
	costDetailCmd.Flags().IntVarP(&costLimit, "limit", "n", 20, "Number of records to show")

	costCmd.AddCommand(costSummaryCmd, costDetailCmd, costBudgetCmd, costSetBudgetCmd)

	rootCmd.AddCommand(initCmd, chatCmd, configCmd, soulCmd, modelsCmd, versionCmd,
		profileCmd, backupCmd, dashboardCmd, debugCmd,
		gatewayCmd, msgGatewayCmd, subCmd, usageCmd, serveCmd, ragCmd, pluginCmd, metricsCmd, wsCmd, agentCmd, evalCmd, tmplCmd, costCmd)

	return rootCmd
}
