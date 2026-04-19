package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/provider"
)

var (
	soulFile  string
	provider_ string
	model_   string
	yolo     bool
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
	soulCmd.AddCommand(soulShowCmd)

	// version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🍀 LuckyHarness v0.5.0")
		},
	}

	// models
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "列出可用模型",
		RunE:  runModels,
	}

	rootCmd.AddCommand(initCmd, chatCmd, configCmd, soulCmd, modelsCmd, versionCmd)

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
	fmt.Println("  lh config set api_key sk-xxx    # 设置 API Key")
	fmt.Println("  lh config set provider openai   # 设置提供商")
	fmt.Println("  lh chat                         # 进入交互模式")
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
