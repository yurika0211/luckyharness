package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Dashboard 是内嵌的 Web Dashboard
type Dashboard struct {
	mu      sync.RWMutex
	server  *http.Server
	addr    string
	running bool

	// 数据提供者（由外部注入）
	providers []DataProvider
}

// DataProvider 提供数据给 Dashboard
type DataProvider interface {
	DashboardData() map[string]interface{}
}

// Config Dashboard 配置
type Config struct {
	Addr string `yaml:"addr,omitempty"` // 默认 :8765
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Addr: ":8765",
	}
}

// New 创建 Dashboard
func New(cfg Config) *Dashboard {
	addr := cfg.Addr
	if addr == "" {
		addr = ":8765"
	}

	return &Dashboard{
		addr:      addr,
		providers: make([]DataProvider, 0),
	}
}

// AddProvider 添加数据提供者
func (d *Dashboard) AddProvider(p DataProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.providers = append(d.providers, p)
}

// Start 启动 Dashboard HTTP 服务
func (d *Dashboard) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("dashboard already running")
	}

	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/data", d.handleData)
	mux.HandleFunc("/api/health", d.handleHealth)

	// 静态文件（内嵌 SPA）
	mux.HandleFunc("/", d.handleSPA)

	d.server = &http.Server{
		Addr:         d.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	d.running = true
	d.mu.Unlock()

	go func() {
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error: %v\n", err)
		}
	}()

	fmt.Printf("🌐 Dashboard running at http://localhost%s\n", d.addr)
	return nil
}

// Stop 停止 Dashboard
func (d *Dashboard) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running || d.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown dashboard: %w", err)
	}

	d.running = false
	return nil
}

// IsRunning 返回 Dashboard 是否运行中
func (d *Dashboard) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// Addr 返回 Dashboard 监听地址
func (d *Dashboard) Addr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.addr
}

// handleStatus 返回系统状态
func (d *Dashboard) handleStatus(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	status := map[string]interface{}{
		"running":   d.running,
		"addr":      d.addr,
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "v0.9.0",
	}

	// 收集所有 provider 数据
	for _, p := range d.providers {
		for k, v := range p.DashboardData() {
			status[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleData 返回详细数据
func (d *Dashboard) handleData(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	data := make(map[string]interface{})
	for _, p := range d.providers {
		for k, v := range p.DashboardData() {
			data[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// handleHealth 健康检查
func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleSPA 提供内嵌 SPA 页面
func (d *Dashboard) handleSPA(w http.ResponseWriter, r *http.Request) {
	// 尝试从静态目录读取
	staticDir := os.Getenv("LH_DASHBOARD_STATIC")
	if staticDir == "" {
		home, _ := os.UserHomeDir()
		staticDir = filepath.Join(home, ".luckyharness", "dashboard")
	}

	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}

	filePath := filepath.Join(staticDir, path)
	data, err := os.ReadFile(filePath)
	if err != nil {
		// 回退到内嵌 HTML
		d.serveEmbeddedSPA(w, r)
		return
	}

	contentType := "text/plain"
	switch {
	case strings.HasSuffix(path, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		contentType = "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		contentType = "application/json; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

// serveEmbeddedSPA 提供内嵌的最小 SPA
func (d *Dashboard) serveEmbeddedSPA(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>🍀 LuckyHarness Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
  .header { background: linear-gradient(135deg, #1e293b 0%, #0f172a 100%); padding: 24px 32px; border-bottom: 1px solid #1e293b; }
  .header h1 { font-size: 24px; color: #22c55e; }
  .header .subtitle { color: #94a3b8; font-size: 14px; margin-top: 4px; }
  .container { max-width: 1200px; margin: 0 auto; padding: 24px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; margin-top: 16px; }
  .card { background: #1e293b; border-radius: 12px; padding: 20px; border: 1px solid #334155; }
  .card h3 { color: #22c55e; font-size: 14px; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 12px; }
  .card .value { font-size: 28px; font-weight: 700; color: #f8fafc; }
  .card .label { font-size: 12px; color: #94a3b8; margin-top: 4px; }
  .status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; }
  .status-dot.green { background: #22c55e; }
  .status-dot.yellow { background: #eab308; }
  .status-dot.red { background: #ef4444; }
  .section { margin-top: 24px; }
  .section h2 { font-size: 18px; color: #f8fafc; margin-bottom: 12px; }
  .log { background: #0f172a; border-radius: 8px; padding: 16px; font-family: monospace; font-size: 13px; max-height: 300px; overflow-y: auto; border: 1px solid #334155; }
  .log .entry { padding: 4px 0; border-bottom: 1px solid #1e293b; }
  .log .time { color: #64748b; }
  .log .msg { color: #e2e8f0; }
  .refresh { background: #22c55e; color: #0f172a; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-weight: 600; }
  .refresh:hover { background: #16a34a; }
  .auto-refresh { margin-left: 12px; color: #94a3b8; font-size: 13px; }
</style>
</head>
<body>
<div class="header">
  <h1>🍀 LuckyHarness Dashboard</h1>
  <div class="subtitle">Go 版自主 AI Agent 框架 — v0.9.0</div>
</div>
<div class="container">
  <div style="display:flex;align-items:center;gap:12px;">
    <button class="refresh" onclick="refresh()">刷新</button>
    <label class="auto-refresh"><input type="checkbox" id="autoRefresh" checked> 自动刷新 (5s)</label>
  </div>
  <div class="grid" id="cards"></div>
  <div class="section">
    <h2>📊 详细数据</h2>
    <div class="log" id="dataLog">加载中...</div>
  </div>
</div>
<script>
let autoRefreshTimer = null;
async function refresh() {
  try {
    const res = await fetch('/api/status');
    const data = await res.json();
    renderCards(data);
    renderData(data);
  } catch(e) {
    document.getElementById('dataLog').textContent = '连接失败: ' + e.message;
  }
}
function renderCards(data) {
  const cards = document.getElementById('cards');
  const items = [
    { title: '状态', value: data.running ? '运行中' : '已停止', dot: data.running ? 'green' : 'red' },
    { title: '地址', value: data.addr || 'N/A', dot: 'green' },
    { title: '活跃 Profile', value: data.active_profile || 'default', dot: 'green' },
    { title: 'Provider', value: data.provider || 'N/A', dot: 'yellow' },
    { title: '模型', value: data.model || 'N/A', dot: 'yellow' },
    { title: '记忆条数', value: (data.memory_short||0)+(data.memory_medium||0)+(data.memory_long||0), dot: 'green' },
  ];
  cards.innerHTML = items.map(i => '<div class="card"><h3>'+i.title+'</h3><div class="value"><span class="status-dot '+i.dot+'"></span>'+i.value+'</div></div>').join('');
}
function renderData(data) {
  const log = document.getElementById('dataLog');
  const entries = Object.entries(data).map(([k,v]) => '<div class="entry"><span class="time">'+new Date().toLocaleTimeString()+'</span> <span class="msg">'+k+': '+JSON.stringify(v)+'</span></div>');
  log.innerHTML = entries.join('');
}
function toggleAutoRefresh() {
  if (autoRefreshTimer) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; }
  if (document.getElementById('autoRefresh').checked) { autoRefreshTimer = setInterval(refresh, 5000); }
}
document.getElementById('autoRefresh').addEventListener('change', toggleAutoRefresh);
refresh();
toggleAutoRefresh();
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, fs.ModeDir|0700)
}
