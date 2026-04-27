package lhcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	"github.com/yurika0211/luckyharness/internal/gateway/onebot"
	"github.com/yurika0211/luckyharness/internal/gateway/telegram"
	"github.com/yurika0211/luckyharness/internal/health"
	"github.com/yurika0211/luckyharness/internal/logger"
	"github.com/yurika0211/luckyharness/internal/profile"
	"github.com/yurika0211/luckyharness/internal/prompt"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/server"
	"github.com/yurika0211/luckyharness/internal/soul"
	"github.com/yurika0211/luckyharness/internal/tool"
)

var (
	soulFile  string
	provider_ string
	model_    string
	yolo      bool
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
	cfg := mgr.Get()
	agent.ApplyAgentLoopConfig(&loopCfg, cfg.Agent)
	if cmd.Flags().Changed("yolo") {
		loopCfg.AutoApprove = yolo
	}

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
	case "msg_gateway.platform":
		fmt.Println(cfg.MsgGateway.Platform)
	case "msg_gateway.api_addr":
		fmt.Println(cfg.MsgGateway.APIAddr)
	case "msg_gateway.telegram.proxy":
		fmt.Println(cfg.MsgGateway.Telegram.Proxy)
	case "msg_gateway.telegram.show_tool_details_in_result", "msg_gateway.telegram.show_tool_chain":
		fmt.Println(cfg.MsgGateway.Telegram.ShowToolDetailsInResult)
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
	mgr, err := config.NewManager()
	if err != nil {
		return err
	}
	if err := mgr.Load(); err != nil {
		return err
	}

	addr, _ := cmd.Flags().GetString("addr")
	if !cmd.Flags().Changed("addr") {
		if cfgAddr := mgr.Get().Dashboard.Addr; cfgAddr != "" {
			addr = cfgAddr
		}
	}

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
	cfg := a.Config().Get()

	gm := a.MsgGateway()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// v0.36.0: 同时启动 HTTP API Server
	apiAddr, _ := cmd.Flags().GetString("api-addr")
	if !cmd.Flags().Changed("api-addr") && cfg.MsgGateway.APIAddr != "" {
		apiAddr = cfg.MsgGateway.APIAddr
	}
	if apiAddr == "" {
		apiAddr = "127.0.0.1:9090"
	}
	rateLimit := cfg.Server.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60
	}
	srv := server.New(a, server.ServerConfig{
		Addr:       apiAddr,
		EnableCORS: cfg.Server.EnableCORS,
		RateLimit:  rateLimit,
	})
	if err := srv.Start(); err != nil {
		return fmt.Errorf("start http api server: %w", err)
	}
	fmt.Printf("📡 HTTP API server starting on %s\n", apiAddr)

	startAll, _ := cmd.Flags().GetBool("all")
	if !cmd.Flags().Changed("all") {
		startAll = cfg.MsgGateway.StartAll
	}
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
	if !cmd.Flags().Changed("platform") && cfg.MsgGateway.Platform != "" {
		platform = cfg.MsgGateway.Platform
	}
	token, _ := cmd.Flags().GetString("token")
	if !cmd.Flags().Changed("token") {
		if cfg.MsgGateway.Telegram.Token != "" {
			token = cfg.MsgGateway.Telegram.Token
		} else if cfg.MsgGateway.Token != "" {
			token = cfg.MsgGateway.Token
		}
	}

	switch platform {
	case "telegram":
		if token == "" {
			return fmt.Errorf("telegram 需要 --token 参数（或在 config.json 里设置 msg_gateway.telegram.token）")
		}
		tgAdapter := telegram.NewAdapter(telegram.Config{
			Token: token,
			Proxy: cfg.MsgGateway.Telegram.Proxy,
		})
		handler := telegram.NewHandler(tgAdapter, a)
		// 持久化 chatID→sessionID 映射，重启后恢复会话
		handler.SetDataDir(filepath.Join(a.Config().HomeDir(), "data", "telegram"))
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

	case "onebot":
		apiBase, _ := cmd.Flags().GetString("onebot-api")
		if !cmd.Flags().Changed("onebot-api") && cfg.MsgGateway.OneBot.APIBase != "" {
			apiBase = cfg.MsgGateway.OneBot.APIBase
		}
		wsURL, _ := cmd.Flags().GetString("onebot-ws")
		if !cmd.Flags().Changed("onebot-ws") && cfg.MsgGateway.OneBot.WSURL != "" {
			wsURL = cfg.MsgGateway.OneBot.WSURL
		}
		obToken, _ := cmd.Flags().GetString("onebot-token")
		if !cmd.Flags().Changed("onebot-token") && cfg.MsgGateway.OneBot.AccessToken != "" {
			obToken = cfg.MsgGateway.OneBot.AccessToken
		}
		botID, _ := cmd.Flags().GetString("onebot-bot-id")
		if !cmd.Flags().Changed("onebot-bot-id") && cfg.MsgGateway.OneBot.BotID != "" {
			botID = cfg.MsgGateway.OneBot.BotID
		}
		showTyping, _ := cmd.Flags().GetBool("onebot-typing")
		if !cmd.Flags().Changed("onebot-typing") {
			showTyping = cfg.MsgGateway.OneBot.ShowTyping
		}
		autoLike, _ := cmd.Flags().GetBool("onebot-like")
		if !cmd.Flags().Changed("onebot-like") {
			autoLike = cfg.MsgGateway.OneBot.AutoLike
		}
		likeTimes, _ := cmd.Flags().GetInt("onebot-like-times")
		if !cmd.Flags().Changed("onebot-like-times") && cfg.MsgGateway.OneBot.LikeTimes > 0 {
			likeTimes = cfg.MsgGateway.OneBot.LikeTimes
		}

		if apiBase == "" {
			return fmt.Errorf("onebot 需要 --onebot-api 参数（或在 config.json 里设置 msg_gateway.onebot.api_base）")
		}

		obAdapter := onebot.NewAdapter(onebot.Config{
			APIBase:       apiBase,
			WSURL:         wsURL,
			AccessToken:   obToken,
			BotQQID:       botID,
			ShowTyping:    showTyping,
			AutoLike:      autoLike,
			LikeTimes:     likeTimes,
			MaxMessageLen: 4000,
		})
		obHandler := onebot.NewHandler(obAdapter, a)
		obAdapter.SetHandler(func(ctx context.Context, msg *gateway.Message) error {
			return obHandler.HandleMessage(ctx, msg)
		})
		if err := gm.Register(obAdapter); err != nil {
			return err
		}
		if err := gm.Start(ctx, "onebot"); err != nil {
			return err
		}
		fmt.Println("✅ OneBot (QQ) 网关已启动")
		if showTyping {
			fmt.Println("   📝 正在输入提示: 开启")
		}
		if autoLike {
			fmt.Printf("   👍 自动点赞: 开启 (%d次)\n", likeTimes)
		}

	default:
		if platform == "" {
			return fmt.Errorf("请通过 --platform 指定平台，或在 config.json 设置 msg_gateway.platform")
		}
		return fmt.Errorf("不支持的平台: %s (支持: telegram, onebot)", platform)
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

type msgGatewayListResponse struct {
	Gateways []gateway.GatewayStatus `json:"gateways"`
	Count    int                     `json:"count"`
}

type msgGatewayErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details"`
}

func runMsgGatewayStop(cmd *cobra.Command, args []string) error {
	baseURL, err := resolveMsgGatewayAPIBase(cmd)
	if err != nil {
		return err
	}

	statuses, err := fetchMsgGatewayStatuses(baseURL)
	if err != nil {
		return fmt.Errorf("无法连接到消息网关 API (%s): %w\n提示: 先运行 `lh msg-gateway start ...`", baseURL, err)
	}

	// stop all running gateways
	if len(args) == 0 {
		if len(statuses) == 0 {
			fmt.Println("📋 暂无已注册的消息网关")
			return nil
		}
		stopped := 0
		for _, st := range statuses {
			if !st.Running {
				continue
			}
			if err := stopMsgGateway(baseURL, st.Name); err != nil {
				return err
			}
			stopped++
		}
		if stopped == 0 {
			fmt.Println("ℹ️ 没有运行中的消息网关")
			return nil
		}
		fmt.Printf("✅ 已停止 %d 个消息网关\n", stopped)
		return nil
	}

	if err := stopMsgGateway(baseURL, args[0]); err != nil {
		return err
	}
	fmt.Printf("✅ 消息网关 %s 已停止\n", args[0])
	return nil
}

func runMsgGatewayStatus(cmd *cobra.Command, args []string) error {
	baseURL, err := resolveMsgGatewayAPIBase(cmd)
	if err != nil {
		return err
	}

	statuses, err := fetchMsgGatewayStatuses(baseURL)
	if err != nil {
		return fmt.Errorf("无法连接到消息网关 API (%s): %w\n提示: 先运行 `lh msg-gateway start ...`", baseURL, err)
	}

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

func resolveMsgGatewayAPIBase(cmd *cobra.Command) (string, error) {
	addr, _ := cmd.Flags().GetString("api-addr")
	addr = strings.TrimSpace(addr)
	if addr == "" {
		mgr, err := config.NewManager()
		if err != nil {
			return "", err
		}
		if err := mgr.Load(); err != nil {
			return "", err
		}
		addr = strings.TrimSpace(mgr.Get().MsgGateway.APIAddr)
	}
	if addr == "" {
		addr = "127.0.0.1:9090"
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return strings.TrimRight(addr, "/"), nil
}

func fetchMsgGatewayStatuses(baseURL string) ([]gateway.GatewayStatus, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/gateways", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeMsgGatewayAPIError(resp)
	}

	var data msgGatewayListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析网关状态失败: %w", err)
	}
	return data.Gateways, nil
}

func stopMsgGateway(baseURL, name string) error {
	client := &http.Client{Timeout: 8 * time.Second}
	reqURL := fmt.Sprintf("%s/api/v1/gateways/%s/stop", baseURL, url.PathEscape(name))
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeMsgGatewayAPIError(resp)
	}
	return nil
}

func decodeMsgGatewayAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr msgGatewayErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
		if apiErr.Details != "" {
			return fmt.Errorf("%s: %s (HTTP %d)", apiErr.Error, apiErr.Details, resp.StatusCode)
		}
		return fmt.Errorf("%s (HTTP %d)", apiErr.Error, resp.StatusCode)
	}
	if len(body) == 0 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
	fmt.Printf("  每日限额: ")
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
	appCfg := mgr.Get()

	addr, _ := cmd.Flags().GetString("addr")
	if !cmd.Flags().Changed("addr") && appCfg.Server.Addr != "" {
		addr = appCfg.Server.Addr
	}
	apiKeys, _ := cmd.Flags().GetStringSlice("api-keys")
	if !cmd.Flags().Changed("api-keys") && len(appCfg.Server.APIKeys) > 0 {
		apiKeys = append([]string(nil), appCfg.Server.APIKeys...)
	}
	noCORS, _ := cmd.Flags().GetBool("no-cors")
	enableCORS := !noCORS
	if !cmd.Flags().Changed("no-cors") {
		enableCORS = appCfg.Server.EnableCORS
	}
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	if !cmd.Flags().Changed("rate-limit") && appCfg.Server.RateLimit > 0 {
		rateLimit = appCfg.Server.RateLimit
	}
	metricsAddr, _ := cmd.Flags().GetString("metrics-addr")
	if !cmd.Flags().Changed("metrics-addr") && appCfg.Server.MetricsAddr != "" {
		metricsAddr = appCfg.Server.MetricsAddr
	}
	logLevel, _ := cmd.Flags().GetString("log-level")
	if !cmd.Flags().Changed("log-level") && appCfg.Server.LogLevel != "" {
		logLevel = appCfg.Server.LogLevel
	}
	logFormat, _ := cmd.Flags().GetString("log-format")
	if !cmd.Flags().Changed("log-format") && appCfg.Server.LogFormat != "" {
		logFormat = appCfg.Server.LogFormat
	}
	corsOrigins := []string{"*"}
	if len(appCfg.Server.CORSOrigins) > 0 {
		corsOrigins = append([]string(nil), appCfg.Server.CORSOrigins...)
	}

	cfg := server.ServerConfig{
		Addr:        addr,
		APIKeys:     apiKeys,
		EnableCORS:  enableCORS,
		CORSOrigins: corsOrigins,
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
		if err := os.WriteFile(soulPath, []byte(content), 0o644); err != nil {
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
			return fmt.Errorf("AGENT %q 未注册", id)
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
		if err := os.WriteFile(evalOutput, []byte(report), 0o644); err != nil {
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
