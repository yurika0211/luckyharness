package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/backup"
	"github.com/yurika0211/luckyharness/internal/collab"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/cost"
	"github.com/yurika0211/luckyharness/internal/dashboard"
	dbg "github.com/yurika0211/luckyharness/internal/debug"
	"github.com/yurika0211/luckyharness/internal/eval"
	"github.com/yurika0211/luckyharness/internal/gateway"
	"github.com/yurika0211/luckyharness/internal/health"
	"github.com/yurika0211/luckyharness/internal/logger"
	"github.com/yurika0211/luckyharness/internal/profile"
	"github.com/yurika0211/luckyharness/internal/prompt"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/server"
	"github.com/yurika0211/luckyharness/internal/soul"
	"github.com/yurika0211/luckyharness/internal/tool"
	"github.com/yurika0211/luckyharness/internal/gateway/telegram"
)

var (
	soulFile  string
	provider_ string
	model_   string
	yolo     bool
	// eval command flags
	evalFormat    string
	evalThreshold float64
	evalOutput    string
	// template command flags
	tmplVars []string
	// cost command flags
	costProvider string
	costModel    string
	costPeriod   string
	costLimit    int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lh",
		Short: "🍀 LuckyHarness — Go 版自主 AI Agent 框架",
		Long:  "LuckyHarness 是一个用 Go 重写的 AI Agent 框架，参考 Hermes Agent 架构迭代开发。",
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
			fmt.Println("🍀 LuckyHarness v0.38.0")
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
	msgGatewayStartCmd.Flags().String("platform", "", "平台名称 (telegram, discord)")
	msgGatewayStartCmd.Flags().String("token", "", "Bot token")
	msgGatewayStartCmd.Flags().Bool("all", false, "启动所有已配置的网关")
	msgGatewayStartCmd.Flags().String("api-addr", "127.0.0.1:9090", "HTTP API 监听地址")
	msgGatewayStopCmd := &cobra.Command{
		Use:   "stop [platform]",
		Short: "停止消息网关",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMsgGatewayStop,
	}
	msgGatewayStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "查看消息网关状态",
		RunE:  runMsgGatewayStatus,
	}
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

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.InitHome(); err != nil {
		return err
	}
	if err := mgr.Save(); err != nil {
		return err
	}

	fmt.Println("🍀 LuckyHarness 初始化完成！")
	fmt.Printf("   主目录: %s\n", mgr.HomeDir())
	fmt.Println()
	fmt.Println("下一步:")
	fmt.Println("  lh profile list              # 查看 Profile")
	fmt.Println("  lh config set api_key sk-xxx # 设置 API Key")
	fmt.Println("  lh config set provider openai # 设置提供商")
	fmt.Println("  lh chat                      # 进入交互模式")
	return nil
}

func runChat(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	if provider_ != "" {
		mgr.Set("provider", provider_)
	}
	if model_ != "" {
		mgr.Set("model", model_)
	}
	if soulFile != "" {
		mgr.Set("soul_path", soulFile)
	}

	userInput := strings.Join(args, " ")

	if userInput == "" {
		// 交互模式
		return startREPL(mgr)
	}

	// 单次对话模式
	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	loopCfg := agent.DefaultLoopConfig()
	loopCfg.AutoApprove = yolo

	ctx := context.Background()
	result, err := a.RunLoop(ctx, userInput, loopCfg)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}

	fmt.Println(result.Response)

	if len(result.ToolCalls) > 0 {
		fmt.Println()
		for _, tc := range result.ToolCalls {
			fmt.Printf("  🔧 %s → %s\n", tc.Name, truncate(tc.Result, 80))
		}
	}

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	cfg := mgr.Get()
	key := args[0]

	switch key {
	case "provider":
		fmt.Println(cfg.Provider)
	case "api_key":
		if cfg.APIKey != "" {
			fmt.Println(cfg.APIKey[:8] + "...")
		} else {
			fmt.Println("(未设置)")
		}
	case "api_base":
		fmt.Println(cfg.APIBase)
	case "model":
		fmt.Println(cfg.Model)
	case "soul_path":
		fmt.Println(cfg.SoulPath)
	case "max_tokens":
		fmt.Println(cfg.MaxTokens)
	case "temperature":
		fmt.Println(cfg.Temperature)
	default:
		if v, ok := cfg.Extra[key]; ok {
			fmt.Println(v)
		} else {
			fmt.Println("(未设置)")
		}
	}
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}
	if err := mgr.Set(args[0], args[1]); err != nil {
		return err
	}
	if err := mgr.Save(); err != nil {
		return err
	}
	fmt.Printf("✅ %s = %s\n", args[0], args[1])
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	cfg := mgr.Get()
	fmt.Println("📋 LuckyHarness 配置:")
	fmt.Printf("  provider:     %s\n", cfg.Provider)
	fmt.Printf("  api_key:      %s\n", maskKey(cfg.APIKey))
	fmt.Printf("  api_base:     %s\n", cfg.APIBase)
	fmt.Printf("  model:        %s\n", cfg.Model)
	fmt.Printf("  soul_path:    %s\n", cfg.SoulPath)
	fmt.Printf("  max_tokens:   %d\n", cfg.MaxTokens)
	fmt.Printf("  temperature:  %.1f\n", cfg.Temperature)
	return nil
}

func runSoulShow(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}
	a, err := agent.New(mgr)
	if err != nil {
		return err
	}
	fmt.Println(a.Soul().SystemPrompt())
	return nil
}

func maskKey(key string) string {
	if key == "" {
		return "(未设置)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "..."
}

func runModels(cmd *cobra.Command, args []string) error {
	catalog := provider.NewModelCatalog()
	models := catalog.List()

	fmt.Println("📋 LuckyHarness 可用模型:")
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

	fmt.Println()
	fmt.Println("使用: lh chat -m <model-id>  或  /model <model-id>")
	return nil
}

// ===== v0.9.0: Profile 命令实现 =====

func getProfileMgr() (*profile.Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	return profile.NewManager(filepath.Join(home, ".luckyharness"))
}

func runProfileList(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	infos := mgr.ListWithInfo()
	if len(infos) == 0 {
		fmt.Println("📋 暂无 Profile")
		return nil
	}

	fmt.Println("📋 LuckyHarness Profiles:")
	for _, info := range infos {
		active := ""
		if info.Active {
			active = " ← active"
		}
		fmt.Printf("  %-15s %-10s %-20s %s%s\n",
			info.Name, info.Provider, info.Model, info.Description, active)
	}
	fmt.Printf("\n  共 %d 个 Profile\n", len(infos))
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	name := "default"
	if len(args) > 0 {
		name = args[0]
	}

	p, err := mgr.Get(name)
	if err != nil {
		return err
	}

	fmt.Printf("📋 Profile: %s\n", p.Name)
	fmt.Printf("  描述:      %s\n", p.Description)
	fmt.Printf("  Provider:  %s\n", p.Provider)
	fmt.Printf("  Model:     %s\n", p.Model)
	fmt.Printf("  API Base:  %s\n", p.APIBase)
	fmt.Printf("  MaxTokens: %d\n", p.MaxTokens)
	fmt.Printf("  Temp:      %.1f\n", p.Temperature)
	fmt.Printf("  SoulPath:  %s\n", p.SoulPath)

	if len(p.Env) > 0 {
		fmt.Println("  环境变量:")
		for k, v := range p.Env {
			fmt.Printf("    %s = %s\n", k, maskKey(v))
		}
	}

	if len(p.Fallbacks) > 0 {
		fmt.Println("  降级链:")
		for i, fb := range p.Fallbacks {
			fmt.Printf("    %d. %s (%s)\n", i+1, fb.Provider, fb.Model)
		}
	}

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	desc, _ := cmd.Flags().GetString("desc")
	p, err := mgr.Create(args[0], desc)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Profile 已创建: %s\n", p.Name)
	fmt.Printf("   描述: %s\n", p.Description)
	fmt.Println()
	fmt.Println("切换: lh profile switch " + p.Name)
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	if err := mgr.Delete(args[0]); err != nil {
		return err
	}

	fmt.Printf("🗑️ Profile 已删除: %s\n", args[0])
	return nil
}

func runProfileSwitch(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	if err := mgr.Switch(args[0]); err != nil {
		return err
	}

	fmt.Printf("✅ 已切换到 Profile: %s\n", args[0])
	return nil
}

func runProfileEnvSet(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	if err := mgr.SetEnv(args[0], args[1], args[2]); err != nil {
		return err
	}

	fmt.Printf("✅ Profile %s: %s = %s\n", args[0], args[1], args[2])
	return nil
}

func runProfileEnvUnset(cmd *cobra.Command, args []string) error {
	mgr, err := getProfileMgr()
	if err != nil {
		return err
	}

	if err := mgr.UnsetEnv(args[0], args[1]); err != nil {
		return err
	}

	fmt.Printf("✅ Profile %s: %s 已删除\n", args[0], args[1])
	return nil
}

// ===== v0.9.0: Backup 命令实现 =====

func runBackupCreate(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	b := backup.New(filepath.Join(home, ".luckyharness"))

	output, _ := cmd.Flags().GetString("output")
	if err := b.Create(output); err != nil {
		return err
	}

	backups, _ := b.List()
	if len(backups) > 0 {
		fmt.Printf("✅ 备份已创建: %s\n", backups[len(backups)-1])
	}
	return nil
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	b := backup.New(filepath.Join(home, ".luckyharness"))

	fmt.Println("⚠️  恢复将覆盖当前数据，是否继续？(y/N)")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("已取消")
		return nil
	}

	if err := b.Restore(args[0]); err != nil {
		return err
	}

	fmt.Println("✅ 备份已恢复")
	return nil
}

func runBackupList(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	b := backup.New(filepath.Join(home, ".luckyharness"))

	backups, err := b.List()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		fmt.Println("📋 暂无备份")
		return nil
	}

	fmt.Println("📋 备份列表:")
	for _, path := range backups {
		info, _ := b.Info(path)
		fmt.Printf("  %s (%v bytes, %v)\n",
			filepath.Base(path), info["size"], info["modTime"])
	}
	return nil
}

// ===== v0.9.0: Dashboard 命令实现 =====

func runDashboardStart(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	cfg := dashboard.Config{Addr: addr}
	d := dashboard.New(cfg)

	if err := d.Start(); err != nil {
		return err
	}

	fmt.Println("按 Ctrl+C 停止 Dashboard...")

	// 阻塞等待信号
	select {}
}

// ===== v0.9.0: Debug 命令实现 =====

func runDebugShare(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	collector := dbg.New(filepath.Join(home, ".luckyharness"))

	opts := dbg.DefaultCollectOptions()
	noEnv, _ := cmd.Flags().GetBool("no-env")
	noConfig, _ := cmd.Flags().GetBool("no-config")
	noLogs, _ := cmd.Flags().GetBool("no-logs")
	opts.Env = !noEnv
	opts.Config = !noConfig
	opts.Logs = !noLogs

	output, _ := cmd.Flags().GetString("output")
	path, err := collector.Export(opts, output)
	if err != nil {
		return err
	}

	fmt.Printf("✅ 调试信息已导出: %s\n", path)
	return nil
}

// ===== v0.10.0: Gateway 命令实现 =====

func getAgent() (*agent.Agent, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, err
	}
	if err := mgr.Load(); err != nil {
		return nil, err
	}
	return agent.New(mgr)
}

func runGatewayInfo(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	gw := a.Gateway()
	tools := a.Tools()

	fmt.Println("🔀 LuckyHarness 工具网关")
	fmt.Printf("  注册工具: %d\n", tools.Count())
	fmt.Println()

	// 按分类列出
	for _, cat := range []tool.Category{tool.CatBuiltin, tool.CatSkill, tool.CatMCP, tool.CatDelegate} {
		tools := tools.ListByCategory(cat)
		if len(tools) == 0 {
			continue
		}
		fmt.Printf("  [%s]\n", cat)
		for _, t := range tools {
			status := "✅"
			if !t.Enabled {
				status = "❌"
			}
			fmt.Printf("    %s %s: %s\n", status, t.Name, t.Description)
		}
	}

	fmt.Println()
	fmt.Println(gw.Router().FormatRoutes())
	return nil
}

func runGatewayRouteList(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	routes := a.Gateway().Router().ListRoutes()
	if len(routes) == 0 {
		fmt.Println("📋 暂无路由规则")
		return nil
	}

	fmt.Println("📋 路由规则:")
	for _, r := range routes {
		status := "✅"
		if !r.Enabled {
			status = "❌"
		}
		fmt.Printf("  %s [%d] %s: %s → %s\n", status, r.Priority, r.Name, r.ToolPattern, r.Target)
	}
	return nil
}

func runGatewayRouteAdd(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	var priority int
	fmt.Sscanf(args[3], "%d", &priority)

	a.Gateway().Router().AddRoute(tool.RouteRule{
		Name:        args[0],
		Priority:    priority,
		ToolPattern: args[1],
		Target:      args[2],
		Enabled:     true,
	})

	fmt.Printf("✅ 路由规则已添加: %s (%s → %s)\n", args[0], args[1], args[2])
	return nil
}

func runGatewayRouteRemove(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	a.Gateway().Router().RemoveRoute(args[0])
	fmt.Printf("🗑️ 路由规则已移除: %s\n", args[0])
	return nil
}

func runGatewayAliasList(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	aliases := a.Gateway().Router().ListAliases()
	if len(aliases) == 0 {
		fmt.Println("📋 暂无别名")
		return nil
	}

	fmt.Println("📋 工具别名:")
	for alias, target := range aliases {
		fmt.Printf("  %s → %s\n", alias, target)
	}
	return nil
}

func runGatewayAliasAdd(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	if err := a.Gateway().Router().AddAlias(args[0], args[1]); err != nil {
		return err
	}

	fmt.Printf("✅ 别名已添加: %s → %s\n", args[0], args[1])
	return nil
}

func runGatewayAliasRemove(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	a.Gateway().Router().RemoveAlias(args[0])
	fmt.Printf("🗑️ 别名已移除: %s\n", args[0])
	return nil
}

// ===== v0.6.0: Messaging Gateway 命令实现 =====

func runMsgGatewayStart(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	gm := a.MsgGateway()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// v0.36.0: 同时启动 HTTP API Server
	apiAddr, _ := cmd.Flags().GetString("api-addr")
	if apiAddr == "" {
		apiAddr = "127.0.0.1:9090"
	}
	srv := server.New(a, server.ServerConfig{
		Addr:       apiAddr,
		EnableCORS: false,
		RateLimit:  60,
	})
	go func() {
		if err := srv.Start(); err != nil {
			fmt.Printf("[server] HTTP API error: %v\n", err)
		}
	}()
	fmt.Printf("📡 HTTP API server starting on %s\n", apiAddr)

	startAll, _ := cmd.Flags().GetBool("all")
	if startAll {
		if err := gm.StartAll(ctx); err != nil {
			return err
		}
		fmt.Println("✅ 所有消息网关已启动")
		// Block until context is cancelled (SIGINT etc.)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\n🛑 正在停止所有消息网关...")
		_ = gm.StopAll()
		_ = srv.Stop()
		return nil
	}

	platform, _ := cmd.Flags().GetString("platform")
	token, _ := cmd.Flags().GetString("token")

	switch platform {
	case "telegram":
		if token == "" {
			return fmt.Errorf("telegram 需要 --token 参数")
		}
		tgAdapter := telegram.NewAdapter(telegram.Config{Token: token})
		handler := telegram.NewHandler(tgAdapter, a)
		tgAdapter.SetHandler(func(ctx context.Context, msg *gateway.Message) error {
			return handler.HandleMessage(ctx, msg)
		})
		if err := gm.Register(tgAdapter); err != nil {
			return err
		}
		if err := gm.Start(ctx, "telegram"); err != nil {
			return err
		}
		fmt.Println("✅ Telegram 网关已启动")
	default:
		return fmt.Errorf("不支持的平台: %s (支持: telegram)", platform)
	}

	// Block until context is cancelled (SIGINT etc.)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\n🛑 正在停止消息网关...")
	_ = gm.StopAll()
	_ = srv.Stop()
	return nil
}

func runMsgGatewayStop(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	gm := a.MsgGateway()

	if len(args) == 0 {
		if err := gm.StopAll(); err != nil {
			return err
		}
		fmt.Println("✅ 所有消息网关已停止")
		return nil
	}

	if err := gm.Stop(args[0]); err != nil {
		return err
	}
	fmt.Printf("✅ 消息网关 %s 已停止\n", args[0])
	return nil
}

func runMsgGatewayStatus(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	gm := a.MsgGateway()
	statuses := gm.Status()

	if len(statuses) == 0 {
		fmt.Println("📋 暂无已注册的消息网关")
		return nil
	}

	fmt.Println("📋 消息网关状态:")
	for _, s := range statuses {
		running := "❌ 停止"
		if s.Running {
			running = "✅ 运行中"
		}
		fmt.Printf("  %s %s (发送: %d, 接收: %d, 错误: %d)\n",
			running, s.Name, s.Stats.MessagesSent, s.Stats.MessagesReceived, s.Stats.Errors)
	}
	return nil
}

// ===== v0.10.0: Subscription 命令实现 =====

func parseDuration(s string) (time.Duration, error) {
	// 支持 30d, 7d, 1h, 24h 等格式
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var num int
	if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c (use d/h/m)", unit)
	}
}

func runSubList(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	subs := a.Gateway().Subscriptions().ListSubscriptions()
	if len(subs) == 0 {
		fmt.Println("📋 暂无订阅")
		return nil
	}

	fmt.Println("📋 订阅列表:")
	for _, s := range subs {
		active := "✅"
		if !s.IsActive() {
			active = "❌"
		}
		fmt.Printf("  %s %s: %s (到期: %s)\n", active, s.UserID, s.Tier, s.ExpiresAt.Format("2006-01-02"))
	}
	return nil
}

func runSubInfo(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	sub := a.Gateway().Subscriptions().GetSubscription(args[0])
	if sub == nil {
		fmt.Printf("📋 用户 %s 无订阅 (Free 级)\n", args[0])
		return nil
	}

	fmt.Printf("📋 订阅详情: %s\n", args[0])
	fmt.Printf("  等级:     %s\n", sub.Tier)
	fmt.Printf("  开始时间: %s\n", sub.StartedAt.Format("2006-01-02 15:04"))
	fmt.Printf("  到期时间: %s\n", sub.ExpiresAt.Format("2006-01-02 15:04"))
	fmt.Printf("  状态:     %s\n", map[bool]string{true: "✅ 有效", false: "❌ 已过期"}[sub.IsActive()])
	fmt.Printf("  自动续费: %v\n", sub.AutoRenew)

	// 显示等级配置
	config := a.Gateway().Subscriptions().GetTierConfig(sub.Tier)
	fmt.Printf("  每日限额: ", )
	if config.MaxCallsPerDay == 0 {
		fmt.Println("无限")
	} else {
		fmt.Printf("%d\n", config.MaxCallsPerDay)
	}
	fmt.Printf("  每小时限额: ")
	if config.MaxCallsPerHour == 0 {
		fmt.Println("无限")
	} else {
		fmt.Printf("%d\n", config.MaxCallsPerHour)
	}
	return nil
}

func runSubSubscribe(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	tier, err := tool.ParseSubTier(args[1])
	if err != nil {
		return err
	}

	duration, err := parseDuration(args[2])
	if err != nil {
		return err
	}

	if err := a.Gateway().Subscriptions().Subscribe(args[0], tier, duration); err != nil {
		return err
	}

	fmt.Printf("✅ 用户 %s 已订阅 %s (%s)\n", args[0], tier, args[2])
	return nil
}

func runSubUnsubscribe(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	a.Gateway().Subscriptions().Unsubscribe(args[0])
	fmt.Printf("🗑️ 用户 %s 已取消订阅\n", args[0])
	return nil
}

// ===== v0.10.0: Usage 命令实现 =====

func runUsageStats(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	stats := a.Gateway().Tracker().GetAllUsage(args[0])
	if len(stats) == 0 {
		fmt.Printf("📋 用户 %s 暂无使用记录\n", args[0])
		return nil
	}

	fmt.Printf("📊 用户 %s 使用统计:\n", args[0])
	for _, s := range stats {
		fmt.Println(s.Format())
	}
	return nil
}

func runUsageQuotaSet(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	var limit int
	fmt.Sscanf(args[3], "%d", &limit)

	if err := a.Gateway().Tracker().SetQuota(args[0], args[1], args[2], limit); err != nil {
		return err
	}

	fmt.Printf("✅ 配额已设置: %s/%s = %d/%s\n", args[0], args[1], limit, args[2])
	return nil
}

func runUsageQuotaList(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	quotas := a.Gateway().Tracker().ListQuotas(args[0])
	if len(quotas) == 0 {
		fmt.Printf("📋 用户 %s 暂无配额限制\n", args[0])
		return nil
	}

	fmt.Printf("📋 用户 %s 配额:\n", args[0])
	for _, q := range quotas {
		fmt.Printf("  %s: %d/%d (%s, 重置: %s)\n",
			q.ToolName, q.Used, q.Limit, q.Window, q.ResetAt.Format("2006-01-02 15:04"))
	}
	return nil
}

func runUsageQuotaRemove(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	a.Gateway().Tracker().RemoveQuota(args[0], args[1])
	fmt.Printf("🗑️ 配额已移除: %s/%s\n", args[0], args[1])
	return nil
}

// ===== v0.13.0: Serve 命令实现 =====

func runServe(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	addr, _ := cmd.Flags().GetString("addr")
	apiKeys, _ := cmd.Flags().GetStringSlice("api-keys")
	noCORS, _ := cmd.Flags().GetBool("no-cors")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	metricsAddr, _ := cmd.Flags().GetString("metrics-addr")
	logLevel, _ := cmd.Flags().GetString("log-level")
	logFormat, _ := cmd.Flags().GetString("log-format")

	cfg := server.ServerConfig{
		Addr:        addr,
		APIKeys:     apiKeys,
		EnableCORS:  !noCORS,
		CORSOrigins: []string{"*"},
		RateLimit:   rateLimit,
		MetricsAddr: metricsAddr,
		LogLevel:    logLevel,
		LogFormat:   logFormat,
	}

	// v0.17.0: 初始化日志
	logger.InitLogger(logger.Config{
		Level:  logLevel,
		Format: logFormat,
	})

	s := server.New(a, cfg)

	// v0.17.0: 注册健康检查
	s.HealthCheck().RegisterCheck("memory", func() health.CheckResult {
		stats := a.MemoryStats()
		if stats == nil {
			return health.CheckResult{Name: "memory", Status: health.StatusDegraded, Error: "memory not initialized"}
		}
		return health.CheckResult{Name: "memory", Status: health.StatusHealthy}
	})
	s.HealthCheck().RegisterCheck("provider", func() health.CheckResult {
		p := a.Provider()
		if p == nil {
			return health.CheckResult{Name: "provider", Status: health.StatusUnhealthy, Error: "no provider configured"}
		}
		return health.CheckResult{Name: "provider", Status: health.StatusHealthy}
	})

	if err := s.Start(); err != nil {
		return err
	}

	fmt.Println("按 Ctrl+C 停止 API Server...")

	// 阻塞等待信号
	select {}
}

// ===== v0.14.0: RAG 命令实现 =====

func runRAGIndex(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	ragMgr := a.RAG()
	path := args[0]

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		docs, err := ragMgr.IndexDirectory(path)
		if err != nil {
			return fmt.Errorf("index directory: %w", err)
		}
		fmt.Printf("✅ Indexed %d documents\n", len(docs))
		for _, d := range docs {
			fmt.Printf("   📄 %s (%d chunks)\n", d.Title, len(d.Chunks))
		}
	} else {
		doc, err := ragMgr.IndexFile(path)
		if err != nil {
			return fmt.Errorf("index file: %w", err)
		}
		fmt.Printf("✅ Indexed: %s (%d chunks)\n", doc.Title, len(doc.Chunks))
	}

	return nil
}

func runRAGSearch(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	ragMgr := a.RAG()
	results, err := ragMgr.Search(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("🔍 No results found")
		return nil
	}

	fmt.Printf("🔍 Found %d results:\n", len(results))
	for i, r := range results {
		content := r.Content
		if len(content) > 120 {
			content = content[:120] + "..."
		}
		fmt.Printf("  %d. [%.2f] %s — %s\n", i+1, r.Score, r.DocTitle, content)
	}

	return nil
}

func runRAGStats(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	stats := a.RAG().Stats()
	fmt.Printf("📚 RAG Knowledge Base:\n")
	fmt.Printf("   Documents: %d\n", stats.DocumentCount)
	fmt.Printf("   Chunks:    %d\n", stats.ChunkCount)
	if !stats.LastIndexed.IsZero() {
		fmt.Printf("   Last indexed: %s\n", stats.LastIndexed.Format("2006-01-02 15:04:05"))
	}
	if len(stats.Sources) > 0 {
		fmt.Println("   Sources:")
		for src, count := range stats.Sources {
			fmt.Printf("     %s: %d chunks\n", src, count)
		}
	}

	return nil
}

// ===== v0.23.0: 流式 RAG 命令实现 =====

func runRAGWatch(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	dir := args[0]
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	streamIndexer.AddWatchDir(dir)
	fmt.Printf("👀 Watching: %s\n", dir)
	return nil
}

func runRAGUnwatch(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	streamIndexer.RemoveWatchDir(args[0])
	fmt.Printf("🚫 Unwatched: %s\n", args[0])
	return nil
}

func runRAGScan(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	changes := streamIndexer.Scan()
	fmt.Printf("🔍 Detected %d changes:\n", len(changes))
	for _, c := range changes {
		fmt.Printf("   %s: %s\n", c.Type, c.Path)
	}
	fmt.Printf("📋 Queue: %d jobs pending\n", streamIndexer.Queue().Len())
	return nil
}

func runRAGStart(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	if streamIndexer.IsRunning() {
		fmt.Println("⚠️  Stream indexer already running")
		return nil
	}

	streamIndexer.Start()
	fmt.Println("▶️  Stream indexer started")
	return nil
}

func runRAGStop(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	if !streamIndexer.IsRunning() {
		fmt.Println("⚠️  Stream indexer not running")
		return nil
	}

	streamIndexer.Stop()
	fmt.Println("⏹️  Stream indexer stopped")
	return nil
}

func runRAGStreamStatus(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		fmt.Println("❌ Stream indexer not initialized")
		return nil
	}

	stats := streamIndexer.Stats()
	fmt.Printf("📊 Stream Indexer Status:\n")
	fmt.Printf("   Running:       %v\n", stats.Running)
	fmt.Printf("   Queue:         %d jobs\n", stats.QueueLen)
	fmt.Printf("   Tracked files: %d\n", stats.TrackedFiles)
	fmt.Printf("   Watch dirs:    %d\n", len(stats.WatchDirs))
	for _, d := range stats.WatchDirs {
		fmt.Printf("     - %s\n", d)
	}
	return nil
}

func runRAGQueue(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	jobs := streamIndexer.Queue().List()
	fmt.Printf("📋 Index Queue (%d jobs):\n", len(jobs))
	for i, job := range jobs {
		fmt.Printf("  %d. [%s] %s (priority %d)\n", i+1, job.JobType, job.Path, job.Priority)
	}
	return nil
}

func runRAGProcess(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	a, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	streamIndexer := a.StreamIndexer()
	if streamIndexer == nil {
		return fmt.Errorf("stream indexer not initialized")
	}

	batch := 1
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &batch)
	}

	jobs, docs, errs := streamIndexer.ProcessBatch(context.Background(), batch)
	fmt.Printf("⚙️  Processed %d jobs:\n", len(jobs))
	for i := range jobs {
		if errs[i] != nil {
			fmt.Printf("  ❌ %s: %v\n", jobs[i].Path, errs[i])
		} else if docs[i] != nil {
			fmt.Printf("  ✅ %s (%d chunks)\n", docs[i].Title, len(docs[i].Chunks))
		} else {
			fmt.Printf("  🗑️  %s (deleted)\n", jobs[i].Path)
		}
	}
	return nil
}

// ===== v0.17.0: Metrics 命令实现 =====

func runMetrics(cmd *cobra.Command, args []string) error {
	// 尝试从运行中的 API Server 获取指标
	addr, _ := cmd.Flags().GetString("addr")
	if addr == "" {
		addr = "http://localhost:9090"
	}
	if !strings.HasPrefix(addr, "http") {
		addr = "http://" + addr
	}

	// 尝试获取 Prometheus 格式指标
	resp, err := http.Get(addr + "/api/v1/metrics")
	if err != nil {
		return fmt.Errorf("无法连接到 API Server (%s): %w\n提示: 先运行 `lh serve` 启动服务器", addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API Server 返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应: %w", err)
	}

	fmt.Println(string(body))
	return nil
}

// ===== v0.18.0: WebSocket 命令实现 =====

func runWSStats(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	if addr == "" {
		addr = "http://localhost:9090"
	}
	if !strings.HasPrefix(addr, "http") {
		addr = "http://" + addr
	}

	resp, err := http.Get(addr + "/api/v1/ws/stats")
	if err != nil {
		return fmt.Errorf("无法连接到 API Server (%s): %w\n提示: 先运行 `lh serve` 启动服务器", addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API Server 返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应: %w", err)
	}

	fmt.Println(string(body))
	return nil
}

func runSoulList(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}
	a, err := agent.New(mgr)
	if err != nil {
		return err
	}

	tm := a.TemplateManager()
	language, _ := cmd.Flags().GetString("language")

	var templates []*soul.Template
	if language != "" {
		templates = tm.ListByLanguage(language)
	} else {
		templates = tm.ListTemplates()
	}

	if len(templates) == 0 {
		fmt.Println("没有可用的 SOUL 模板")
		return nil
	}

	fmt.Printf("%-20s %-12s %-8s %s\n", "ID", "名称", "语言", "描述")
	fmt.Println(strings.Repeat("-", 70))
	for _, t := range templates {
		desc := t.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		fmt.Printf("%-20s %-12s %-8s %s\n", t.ID, t.Name, t.Language, desc)
	}
	return nil
}

func runSoulSwitch(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}
	a, err := agent.New(mgr)
	if err != nil {
		return err
	}

	tm := a.TemplateManager()
	tmpl, err := tm.GetTemplate(templateID)
	if err != nil {
		return fmt.Errorf("模板 %q 不存在: %w", templateID, err)
	}

	// 渲染模板内容作为新 SOUL
	content := tmpl.Render(nil)
	newSoul := &soul.Soul{Content: content}

	// 更新 Agent 的 SOUL
	// 注意：这里直接替换内存中的 SOUL，持久化需要写入文件
	soulPath := mgr.Get().SoulPath
	if soulPath != "" {
		if err := os.WriteFile(soulPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("写入 SOUL 文件失败: %w", err)
		}
		newSoul.FilePath = soulPath
	}

	// 通过反射或直接方式更新 agent 的 soul 字段
	// 由于 Agent.soul 是私有字段，我们通过配置重载来实现
	fmt.Printf("✅ 已切换到 SOUL 模板: %s (%s)\n", tmpl.Name, soul.LanguageName(tmpl.Language))
	fmt.Printf("   语言: %s\n", soul.LanguageName(tmpl.Language))
	if tmpl.Description != "" {
		fmt.Printf("   描述: %s\n", tmpl.Description)
	}
	fmt.Println()
	fmt.Println("--- SOUL 内容预览 ---")
	preview := content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	fmt.Println(preview)

	_ = newSoul // 将来可用于热更新
	return nil
}

// ===== v0.22.0: Agent 协作命令实现 =====

func runAgentList(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	reg := a.AgentRegistry()
	agents := reg.List()

	if len(agents) == 0 {
		fmt.Println("📋 暂无注册的 Agent")
		return nil
	}

	fmt.Printf("%-20s %-12s %-10s %s\n", "ID", "名称", "状态", "能力")
	fmt.Println(strings.Repeat("-", 70))
	for _, p := range agents {
		caps := strings.Join(p.Capabilities, ", ")
		if len(caps) > 30 {
			caps = caps[:27] + "..."
		}
		fmt.Printf("%-20s %-12s %-10s %s\n", p.ID, p.Name, p.Status, caps)
	}

	total, online, busy, offline := reg.Count()
	fmt.Printf("\n统计: 总计 %d | 在线 %d | 忙碌 %d | 离线 %d\n", total, online, busy, offline)
	return nil
}

func runAgentDelegate(cmd *cobra.Command, args []string) error {
	modeStr := args[0]
	input := args[1]
	agentIDs := args[2:]

	mode, err := collab.ParseMode(modeStr)
	if err != nil {
		return fmt.Errorf("无效的协作模式 %q: %w (支持: pipeline, parallel, debate)", modeStr, err)
	}

	a, err := getAgent()
	if err != nil {
		return err
	}

	dm := a.CollabManager()
	if dm == nil {
		return fmt.Errorf("协作管理器未初始化")
	}

	// 验证 Agent 存在
	reg := a.AgentRegistry()
	for _, id := range agentIDs {
		if _, ok := reg.Get(id); !ok {
			return fmt.Errorf("Agent %q 未注册", id)
		}
	}

	fmt.Printf("🚀 创建协作任务 (模式: %s)\n", mode)
	fmt.Printf("   输入: %s\n", input)
	fmt.Printf("   Agent: %s\n", strings.Join(agentIDs, ", "))

	task, err := dm.Delegate(context.Background(), mode, "CLI delegate", input, agentIDs, 60*time.Second)
	if err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	fmt.Printf("\n✅ 任务已创建: %s\n", task.ID)
	fmt.Printf("   状态: %s\n", task.State)

	// 等待完成
	fmt.Println("\n⏳ 等待任务完成...")
	for {
		time.Sleep(500 * time.Millisecond)
		updated, ok := dm.GetTask(task.ID)
		if !ok {
			return fmt.Errorf("任务丢失")
		}
		if updated.State == collab.TaskCompleted || updated.State == collab.TaskFailed || updated.State == collab.TaskCancelled {
			task = updated
			break
		}
		fmt.Printf("   状态: %s\n", updated.State)
	}

	fmt.Printf("\n📋 任务结果:\n")
	fmt.Printf("   状态: %s\n", task.State)
	if task.Result != "" {
		fmt.Printf("   结果: %s\n", task.Result)
	}
	if task.State == collab.TaskFailed {
		for _, sub := range task.SubTasks {
			if sub.Error != "" {
				fmt.Printf("   错误 [%s]: %s\n", sub.AgentID, sub.Error)
			}
		}
	}
	return nil
}

func runAgentTask(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	a, err := getAgent()
	if err != nil {
		return err
	}

	dm := a.CollabManager()
	task, ok := dm.GetTask(taskID)
	if !ok {
		return fmt.Errorf("任务 %q 不存在", taskID)
	}

	fmt.Printf("📋 任务详情: %s\n", task.ID)
	fmt.Printf("   模式: %s\n", task.Mode)
	fmt.Printf("   描述: %s\n", task.Description)
	fmt.Printf("   状态: %s\n", task.State)
	fmt.Printf("   创建: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if !task.CompletedAt.IsZero() {
		fmt.Printf("   完成: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\n子任务 (%d):\n", len(task.SubTasks))
	for _, sub := range task.SubTasks {
		fmt.Printf("  - %s [%s]: %s\n", sub.AgentID, sub.State, sub.Description)
		if sub.Error != "" {
			fmt.Printf("    错误: %s\n", sub.Error)
		}
	}

	if task.Result != "" {
		fmt.Printf("\n结果:\n%s\n", task.Result)
	}
	return nil
}

func runAgentTasks(cmd *cobra.Command, args []string) error {
	a, err := getAgent()
	if err != nil {
		return err
	}

	dm := a.CollabManager()
	tasks := dm.ListTasks()

	if len(tasks) == 0 {
		fmt.Println("📋 暂无协作任务")
		return nil
	}

	fmt.Printf("%-20s %-10s %-10s %s\n", "ID", "模式", "状态", "描述")
	fmt.Println(strings.Repeat("-", 70))
	for _, t := range tasks {
		desc := t.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		fmt.Printf("%-20s %-10s %-10s %s\n", t.ID, t.Mode, t.State, desc)
	}

	total, running, completed, failed := dm.Stats()
	fmt.Printf("\n统计: 总计 %d | 运行中 %d | 已完成 %d | 失败 %d\n", total, running, completed, failed)
	return nil
}

func runAgentCancel(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	a, err := getAgent()
	if err != nil {
		return err
	}

	dm := a.CollabManager()
	if err := dm.CancelTask(taskID); err != nil {
		return fmt.Errorf("取消任务失败: %w", err)
	}

	fmt.Printf("✅ 任务 %s 已取消\n", taskID)
	return nil
}

// ---- eval command implementations ----

func runEvalRun(cmd *cobra.Command, args []string) error {
	dir := args[0]

	cases, err := eval.LoadTestCasesFromDir(dir)
	if err != nil {
		return fmt.Errorf("加载测试用例失败: %w", err)
	}
	if len(cases) == 0 {
		return fmt.Errorf("目录 %s 中没有找到测试用例", dir)
	}

	fmt.Printf("📋 加载了 %d 个测试用例\n", len(cases))

	// Use a simple agent runner that delegates to the configured agent
	runner := &cliAgentRunner{agent: nil} // will be initialized on first call
	br := eval.NewBenchmarkRunner(runner, evalThreshold)

	result := br.Run(context.Background(), cases)

	format := eval.ReportFormat(evalFormat)
	report, err := eval.GenerateReport(result, format)
	if err != nil {
		return fmt.Errorf("生成报告失败: %w", err)
	}

	if evalOutput != "" {
		if err := os.WriteFile(evalOutput, []byte(report), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
		fmt.Printf("📊 报告已保存到 %s\n", evalOutput)
	} else {
		fmt.Println(report)
	}

	return nil
}

func runEvalList(cmd *cobra.Command, args []string) error {
	dir := args[0]

	cases, err := eval.LoadTestCasesFromDir(dir)
	if err != nil {
		return fmt.Errorf("加载测试用例失败: %w", err)
	}

	if len(cases) == 0 {
		fmt.Println("没有找到测试用例")
		return nil
	}

	fmt.Printf("📋 共 %d 个测试用例:\n\n", len(cases))
	for _, tc := range cases {
		tags := ""
		if len(tc.Tags) > 0 {
			tags = " [" + strings.Join(tc.Tags, ", ") + "]"
		}
		fmt.Printf("  • %s: %s%s\n", tc.ID, tc.Name, tags)
		if tc.Description != "" {
			fmt.Printf("    %s\n", tc.Description)
		}
	}

	return nil
}

func runEvalReport(cmd *cobra.Command, args []string) error {
	path := args[0]

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取报告失败: %w", err)
	}

	var result eval.BenchmarkResult
	ext := filepath.Ext(path)
	if ext == ".json" {
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("解析 JSON 报告失败: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("解析 YAML 报告失败: %w", err)
		}
	}

	format := eval.ReportFormat(evalFormat)
	report, err := eval.GenerateReport(&result, format)
	if err != nil {
		return fmt.Errorf("生成报告失败: %w", err)
	}

	fmt.Println(report)
	return nil
}

// cliAgentRunner is a simple AgentRunner that uses the LuckyHarness agent.
type cliAgentRunner struct {
	agent *agent.Agent
}

func (r *cliAgentRunner) Run(ctx context.Context, input eval.EvalInput) (eval.EvalOutput, error) {
	// Lazy init
	if r.agent == nil {
		a, err := getAgent()
		if err != nil {
			return eval.EvalOutput{}, fmt.Errorf("初始化 agent 失败: %w", err)
		}
		r.agent = a
	}

	start := time.Now()
	resp, err := r.agent.Chat(ctx, input.Query)
	elapsed := time.Since(start)

	if err != nil {
		return eval.EvalOutput{}, err
	}

	output := eval.EvalOutput{
		Response: resp,
		Latency:  elapsed,
	}

	return output, nil
}

// ---- template command implementations ----

func getTemplateStore() (*prompt.TemplateStore, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, err
	}
	home := mgr.HomeDir()
	tmplDir := filepath.Join(home, "templates")

	store := prompt.NewTemplateStore()
	if _, err := os.Stat(tmplDir); err == nil {
		_ = store.LoadFromDir(tmplDir)
	}
	return store, nil
}

func runTemplateRender(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getTemplateStore()
	if err != nil {
		return fmt.Errorf("初始化模板存储失败: %w", err)
	}

	data := make(prompt.RenderData)
	for _, v := range tmplVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("无效变量格式: %s (期望 key=value)", v)
		}
		data[parts[0]] = parts[1]
	}

	engine := prompt.NewEngine(store)

	result, err := engine.Render(name, data)
	if err != nil {
		return fmt.Errorf("渲染模板失败: %w", err)
	}

	fmt.Println(result)
	return nil
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	store, err := getTemplateStore()
	if err != nil {
		return fmt.Errorf("初始化模板存储失败: %w", err)
	}

	if len(args) > 0 {
		if err := store.LoadFromDir(args[0]); err != nil {
			return fmt.Errorf("加载目录失败: %w", err)
		}
	}

	names := store.List()
	if len(names) == 0 {
		fmt.Println("没有找到模板")
		return nil
	}

	fmt.Printf("📋 共 %d 个模板:\n", len(names))
	for _, name := range names {
		t, _ := store.Get(name)
		layout := ""
		if t != nil && t.Layout != "" {
			layout = fmt.Sprintf(" (layout: %s)", t.Layout)
		}
		fmt.Printf("  • %s%s\n", name, layout)
	}
	return nil
}

func runTemplateValidate(cmd *cobra.Command, args []string) error {
	path := args[0]

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	engine := prompt.NewEngine(prompt.NewTemplateStore())
	if err := engine.Validate(string(data)); err != nil {
		fmt.Printf("❌ 模板验证失败: %v\n", err)
		return nil
	}

	fmt.Printf("✅ 模板验证通过: %s\n", path)
	return nil
}

// ---- cost command implementations ----

func getCostStore() (*cost.CostStore, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, err
	}
	home := mgr.HomeDir()
	costFile := filepath.Join(home, "costs.json")

	prices := cost.NewPriceTable()
	store := cost.NewCostStore(prices)

	if _, err := os.Stat(costFile); err == nil {
		_ = store.Load(costFile)
	}
	store.SetFilePath(costFile)
	return store, nil
}

func runCostSummary(cmd *cobra.Command, args []string) error {
	store, err := getCostStore()
	if err != nil {
		return fmt.Errorf("初始化成本存储失败: %w", err)
	}

	summary := store.Summary(cost.SummaryOptions{
		Provider: costProvider,
		Model:    costModel,
		Period:   costPeriod,
	})

	fmt.Printf("💰 成本汇总 (%s)\n", periodLabel(costPeriod))
	if costProvider != "" {
		fmt.Printf("  Provider: %s\n", costProvider)
	}
	if costModel != "" {
		fmt.Printf("  Model: %s\n", costModel)
	}
	fmt.Printf("  调用次数: %d\n", summary.TotalCalls)
	fmt.Printf("  总 Token: %d (prompt: %d, completion: %d)\n",
		summary.TotalTokens, summary.PromptTokens, summary.CompletionTokens)
	fmt.Printf("  总费用: $%.6f\n", summary.TotalCostUSD)

	// Show breakdown by model
	if costProvider == "" && costModel == "" {
		byModel := store.ByModel(costPeriod)
		if len(byModel) > 0 {
			fmt.Println("\n  📊 按模型分布:")
			for key, s := range byModel {
				fmt.Printf("    %-30s  %d calls  $%.4f\n", key, s.TotalCalls, s.TotalCostUSD)
			}
		}
	}

	return nil
}

func runCostDetail(cmd *cobra.Command, args []string) error {
	store, err := getCostStore()
	if err != nil {
		return fmt.Errorf("初始化成本存储失败: %w", err)
	}

	records := store.Recent(costLimit)
	if len(records) == 0 {
		fmt.Println("没有成本记录")
		return nil
	}

	fmt.Printf("📋 最近 %d 条记录:\n", len(records))
	for _, r := range records {
		fmt.Printf("  %s | %s/%s | tokens: %d+%d=%d | $%.6f\n",
			r.Timestamp.Format("2006-01-02 15:04:05"),
			r.Provider, r.Model,
			r.PromptTokens, r.CompletionTokens, r.TotalTokens,
			r.CostUSD)
	}

	return nil
}

func runCostBudget(cmd *cobra.Command, args []string) error {
	store, err := getCostStore()
	if err != nil {
		return fmt.Errorf("初始化成本存储失败: %w", err)
	}

	bm := cost.NewBudgetManager(store)
	statuses := bm.Status()

	if len(statuses) == 0 {
		fmt.Println("未设置预算。使用 lh cost set-budget <period> <limit-usd> 设置。")
		return nil
	}

	fmt.Println("📊 预算状态:")
	for _, s := range statuses {
		provider := ""
		if s.Config.Provider != "" {
			provider = fmt.Sprintf(" (%s)", s.Config.Provider)
		}
		status := "🟢 正常"
		if s.Percentage >= s.Config.CriticalPct {
			status = "🔴 超支"
		} else if s.Percentage >= s.Config.WarningPct {
			status = "🟡 警告"
		}
		fmt.Printf("  %s%s: $%.4f / $%.2f (%.1f%%) 剩余 $%.4f  %s\n",
			s.Config.Period, provider,
			s.SpentUSD, s.Config.LimitUSD, s.Percentage, s.Remaining, status)
	}

	return nil
}

func runCostSetBudget(cmd *cobra.Command, args []string) error {
	period := args[0]
	var limit float64
	if _, err := fmt.Sscanf(args[1], "%f", &limit); err != nil {
		return fmt.Errorf("无效金额: %s", args[1])
	}

	store, err := getCostStore()
	if err != nil {
		return fmt.Errorf("初始化成本存储失败: %w", err)
	}

	bm := cost.NewBudgetManager(store)
	bm.SetBudget(cost.BudgetConfig{
		Period:   period,
		LimitUSD: limit,
	})

	fmt.Printf("✅ 已设置 %s 预算: $%.2f\n", period, limit)
	return nil
}

func periodLabel(period string) string {
	switch period {
	case "today":
		return "今天"
	case "week":
		return "最近一周"
	case "month":
		return "最近一月"
	case "all":
		return "全部"
	default:
		return period
	}
}
