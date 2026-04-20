package plugin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Installer 插件安装器
type Installer struct {
	registry    *Registry
	pluginsDir  string
	httpClient  *http.Client
}

// NewInstaller 创建插件安装器
func NewInstaller(registry *Registry, pluginsDir string) *Installer {
	return &Installer{
		registry:   registry,
		pluginsDir: pluginsDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InstallResult 安装结果
type InstallResult struct {
	Name       string
	Version    string
	InstallDir string
	Installed  bool
	Updated    bool // true if this was an update
	Error      string
}

// Install 从本地目录安装插件
func (inst *Installer) Install(localPath string) (*InstallResult, error) {
	// 读取 plugin.yaml
	manifestPath := filepath.Join(localPath, "plugin.yaml")
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// 检查是否已安装
	_, alreadyInstalled := inst.registry.Get(manifest.Name)
	isUpdate := alreadyInstalled

	// 创建目标目录
	targetDir := filepath.Join(inst.pluginsDir, manifest.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	// 复制文件
	if err := copyDir(localPath, targetDir); err != nil {
		return nil, fmt.Errorf("copy plugin files: %w", err)
	}

	// 更新清单中的安装路径
	manifest.InstallPath = targetDir
	manifest.InstalledAt = time.Now()

	// 注册到 Registry
	inst.registry.Register(manifest)

	return &InstallResult{
		Name:       manifest.Name,
		Version:    manifest.Version,
		InstallDir: targetDir,
		Installed:  true,
		Updated:    isUpdate,
	}, nil
}

// InstallFromURL 从 URL 下载并安装插件
func (inst *Installer) InstallFromURL(downloadURL string) (*InstallResult, error) {
	// 下载到临时目录
	tmpDir, err := os.MkdirTemp("", "lh-plugin-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 下载
	resp, err := inst.httpClient.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// 保存到临时文件
	tmpFile := filepath.Join(tmpDir, "plugin.zip")
	f, err := os.Create(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return nil, fmt.Errorf("save download: %w", err)
	}
	f.Close()

	// TODO: 解压 zip 并安装
	// 当前版本只支持本地目录安装
	return nil, fmt.Errorf("URL installation not yet implemented (use local path)")
}

// Uninstall 卸载插件
func (inst *Installer) Uninstall(name string) error {
	entry, ok := inst.registry.Get(name)
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// 删除插件目录
	if entry.Manifest.InstallPath != "" {
		if err := os.RemoveAll(entry.Manifest.InstallPath); err != nil {
			return fmt.Errorf("remove plugin dir: %w", err)
		}
	}

	// 从注册表移除
	return inst.registry.Unregister(name)
}

// Update 更新插件（重新安装）
func (inst *Installer) Update(name string, localPath string) (*InstallResult, error) {
	_, ok := inst.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	// 先卸载旧版本
	if err := inst.Uninstall(name); err != nil {
		return nil, fmt.Errorf("uninstall old version: %w", err)
	}

	// 安装新版本
	result, err := inst.Install(localPath)
	if err != nil {
		return nil, fmt.Errorf("install new version: %w", err)
	}

	result.Updated = true
	return result, nil
}

// GetInstalledPath 获取已安装插件的路径
func (inst *Installer) GetInstalledPath(name string) (string, error) {
	entry, ok := inst.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("plugin not found: %s", name)
	}
	if entry.Manifest.InstallPath == "" {
		return "", fmt.Errorf("plugin install path not set: %s", name)
	}
	return entry.Manifest.InstallPath, nil
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// 跳过 .git 目录
			if strings.Contains(relPath, ".git") {
				return nil
			}
			return os.MkdirAll(targetPath, 0755)
		}

		// 跳过 .git 文件
		if strings.Contains(relPath, ".git") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// 保留可执行权限
		perm := info.Mode().Perm()
		return os.WriteFile(targetPath, data, perm)
	})
}