package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/plugin"
)

// ===== v0.15.0: Plugin 命令 =====

func getPluginRegistry() (*plugin.Registry, *plugin.Installer, *plugin.Sandbox, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load config: %w", err)
	}
	pluginsDir := filepath.Join(mgr.HomeDir(), "plugins")

	reg := plugin.NewRegistry(pluginsDir)
	inst := plugin.NewInstaller(reg, pluginsDir)
	sandbox := plugin.NewDefaultSandbox()

	// 从磁盘加载已安装的插件
	if err := reg.LoadFromDisk(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  加载插件警告: %v\n", err)
	}

	// 为已安装的插件注册沙箱
	for _, entry := range reg.List() {
		sandbox.RegisterPlugin(entry.Manifest.Name, entry.Manifest)
	}

	return reg, inst, sandbox, nil
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin install <path>")
	}
	localPath := args[0]

	reg, inst, sandbox, err := getPluginRegistry()
	if err != nil {
		return err
	}
	_ = reg
	_ = sandbox

	result, err := inst.Install(localPath)
	if err != nil {
		return fmt.Errorf("install plugin: %w", err)
	}

	if result.Updated {
		fmt.Printf("✅ 插件 %s 更新到 v%s\n", result.Name, result.Version)
	} else {
		fmt.Printf("✅ 插件 %s v%s 安装成功\n", result.Name, result.Version)
	}
	fmt.Printf("   安装路径: %s\n", result.InstallDir)
	return nil
}

func runPluginList(cmd *cobra.Command, args []string) error {
	reg, _, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	plugins := reg.List()
	if len(plugins) == 0 {
		fmt.Println("📦 没有已安装的插件")
		return nil
	}

	fmt.Println("📦 已安装插件:")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-20s %-10s %-10s %-10s %s\n", "名称", "版本", "类型", "状态", "描述")
	fmt.Println(strings.Repeat("-", 70))

	for _, p := range plugins {
		status := string(p.Status)
		if p.Status == plugin.StatusInstalled {
			status = "✅ 已安装"
		} else if p.Status == plugin.StatusError {
			status = "❌ 错误"
		} else if p.Status == plugin.StatusDisabled {
			status = "⏸️  禁用"
		}
		desc := p.Manifest.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		fmt.Printf("%-20s %-10s %-10s %-10s %s\n",
			p.Manifest.Name, p.Manifest.Version, p.Manifest.Type, status, desc)
	}

	fmt.Printf("\n共 %d 个插件\n", len(plugins))
	return nil
}

func runPluginRemove(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin remove <name>")
	}
	name := args[0]

	_, inst, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	if err := inst.Uninstall(name); err != nil {
		return fmt.Errorf("uninstall plugin: %w", err)
	}

	fmt.Printf("🗑️  插件 %s 已卸载\n", name)
	return nil
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lh plugin update <name> <path>")
	}
	name := args[0]
	localPath := args[1]

	_, inst, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	result, err := inst.Update(name, localPath)
	if err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}

	fmt.Printf("🔄 插件 %s 更新到 v%s\n", result.Name, result.Version)
	return nil
}

func runPluginSearch(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin search <query>")
	}
	query := args[0]

	// 当前版本只搜索本地已安装的插件
	reg, _, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	plugins := reg.List()
	var matches []*plugin.PluginEntry
	for _, p := range plugins {
		// 简单匹配：名称、描述、作者、标签
		lowerQuery := strings.ToLower(query)
		if strings.Contains(strings.ToLower(p.Manifest.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(p.Manifest.Description), lowerQuery) ||
			strings.Contains(strings.ToLower(p.Manifest.Author), lowerQuery) {
			matches = append(matches, p)
			continue
		}
		for _, tag := range p.Manifest.Tags {
			if strings.Contains(strings.ToLower(tag), lowerQuery) {
				matches = append(matches, p)
				break
			}
		}
	}

	if len(matches) == 0 {
		fmt.Printf("🔍 没有找到匹配 '%s' 的插件\n", query)
		return nil
	}

	fmt.Printf("🔍 搜索 '%s' 结果:\n", query)
	fmt.Println(strings.Repeat("-", 60))
	for _, p := range matches {
		fmt.Printf("  %-20s v%-8s %s\n", p.Manifest.Name, p.Manifest.Version, p.Manifest.Description)
	}
	fmt.Printf("\n共 %d 个结果\n", len(matches))
	return nil
}

func runPluginInfo(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin info <name>")
	}
	name := args[0]

	reg, _, sandbox, err := getPluginRegistry()
	if err != nil {
		return err
	}

	entry, ok := reg.Get(name)
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	m := entry.Manifest
	fmt.Printf("📦 插件: %s\n", m.FullName())
	fmt.Printf("   类型: %s\n", m.Type)
	fmt.Printf("   作者: %s\n", m.Author)
	fmt.Printf("   描述: %s\n", m.Description)
	if m.License != "" {
		fmt.Printf("   许可: %s\n", m.License)
	}
	if m.Homepage != "" {
		fmt.Printf("   主页: %s\n", m.Homepage)
	}
	if m.Repository != "" {
		fmt.Printf("   仓库: %s\n", m.Repository)
	}
	if m.InstallPath != "" {
		fmt.Printf("   路径: %s\n", m.InstallPath)
	}
	if m.MinVersion != "" {
		fmt.Printf("   最低版本: %s\n", m.MinVersion)
	}

	// 权限
	perms, _ := sandbox.GetPermissions(name)
	fmt.Printf("   权限: %s\n", plugin.FormatPermissions(perms))

	// 依赖
	if len(m.Dependencies) > 0 {
		fmt.Println("   依赖:")
		for _, dep := range m.Dependencies {
			if dep.Version != "" {
				fmt.Printf("     - %s@%s\n", dep.Name, dep.Version)
			} else {
				fmt.Printf("     - %s\n", dep.Name)
			}
		}
	}

	// 标签
	if len(m.Tags) > 0 {
		fmt.Printf("   标签: %s\n", strings.Join(m.Tags, ", "))
	}

	// 状态
	fmt.Printf("   状态: %s\n", entry.Status)
	if entry.Error != "" {
		fmt.Printf("   错误: %s\n", entry.Error)
	}

	return nil
}

func runPluginEnable(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin enable <name>")
	}
	name := args[0]

	reg, _, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	if err := reg.UpdateStatus(name, plugin.StatusInstalled); err != nil {
		return fmt.Errorf("enable plugin: %w", err)
	}

	fmt.Printf("✅ 插件 %s 已启用\n", name)
	return nil
}

func runPluginDisable(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lh plugin disable <name>")
	}
	name := args[0]

	reg, _, _, err := getPluginRegistry()
	if err != nil {
		return err
	}

	if err := reg.UpdateStatus(name, plugin.StatusDisabled); err != nil {
		return fmt.Errorf("disable plugin: %w", err)
	}

	fmt.Printf("⏸️  插件 %s 已禁用\n", name)
	return nil
}