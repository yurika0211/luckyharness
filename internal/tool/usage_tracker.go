package tool

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// UsageTracker 工具使用计量追踪器
type UsageTracker struct {
	mu     sync.RWMutex
	records map[string][]UsageRecord // userID -> records
	quotas map[string]*Quota         // "userID:toolName" -> quota
}

// UsageRecord 单次使用记录
type UsageRecord struct {
	ToolName  string    `json:"tool_name"`
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Success   bool      `json:"success"`
}

// Quota 工具使用配额
type Quota struct {
	ToolName   string `json:"tool_name"`
	Limit      int    `json:"limit"`       // 最大调用次数 (0 = 无限)
	Window     string `json:"window"`      // 时间窗口: "hourly", "daily", "monthly"
	Used       int    `json:"used"`        // 已使用次数
	ResetAt    time.Time `json:"reset_at"` // 配额重置时间
}

// NewUsageTracker 创建用量追踪器
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		records: make(map[string][]UsageRecord),
		quotas:  make(map[string]*Quota),
	}
}

// Record 记录一次工具使用
func (t *UsageTracker) Record(userID, toolName string, duration time.Duration, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	record := UsageRecord{
		ToolName:  toolName,
		Timestamp: time.Now(),
		Duration:  duration,
		Success:   success,
	}

	t.records[userID] = append(t.records[userID], record)

	// 保留最近 1000 条记录
	if len(t.records[userID]) > 1000 {
		t.records[userID] = t.records[userID][len(t.records[userID])-1000:]
	}
}

// CheckQuota 检查是否还有配额
func (t *UsageTracker) CheckQuota(userID, toolName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := userID + ":" + toolName
	quota, ok := t.quotas[key]
	if !ok {
		return true // 无配额限制
	}

	// 检查是否需要重置
	if time.Now().After(quota.ResetAt) {
		return true // 已过重置时间，配额刷新
	}

	return quota.Used < quota.Limit
}

// SetQuota 设置工具使用配额
func (t *UsageTracker) SetQuota(userID, toolName, window string, limit int) error {
	if limit < 0 {
		return fmt.Errorf("quota limit must be >= 0")
	}

	var resetAt time.Time
	now := time.Now()
	switch window {
	case "hourly":
		resetAt = now.Add(time.Hour)
	case "daily":
		resetAt = now.AddDate(0, 0, 1)
	case "monthly":
		resetAt = now.AddDate(0, 1, 0)
	default:
		return fmt.Errorf("unknown quota window: %s (use hourly/daily/monthly)", window)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	key := userID + ":" + toolName
	t.quotas[key] = &Quota{
		ToolName: toolName,
		Limit:    limit,
		Window:   window,
		Used:     0,
		ResetAt:  resetAt,
	}

	return nil
}

// RemoveQuota 移除配额限制
func (t *UsageTracker) RemoveQuota(userID, toolName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := userID + ":" + toolName
	delete(t.quotas, key)
}

// IncrementUsage 增加使用计数（由 Gateway 调用）
func (t *UsageTracker) IncrementUsage(userID, toolName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := userID + ":" + toolName
	if quota, ok := t.quotas[key]; ok {
		// 检查是否需要重置
		if time.Now().After(quota.ResetAt) {
			quota.Used = 0
			switch quota.Window {
			case "hourly":
				quota.ResetAt = time.Now().Add(time.Hour)
			case "daily":
				quota.ResetAt = time.Now().AddDate(0, 0, 1)
			case "monthly":
				quota.ResetAt = time.Now().AddDate(0, 1, 0)
			}
		}
		quota.Used++
	}
}

// GetUsage 获取用户对某工具的使用统计
func (t *UsageTracker) GetUsage(userID, toolName string) UsageStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records, ok := t.records[userID]
	if !ok {
		return UsageStats{ToolName: toolName}
	}

	var stats UsageStats
	stats.ToolName = toolName
	var totalDuration time.Duration

	for _, r := range records {
		if r.ToolName != toolName {
			continue
		}
		stats.TotalCalls++
		if r.Success {
			stats.SuccessCalls++
		} else {
			stats.FailedCalls++
		}
		totalDuration += r.Duration
		if r.Timestamp.After(stats.LastUsed) {
			stats.LastUsed = r.Timestamp
		}
	}

	if stats.TotalCalls > 0 {
		stats.AvgDuration = totalDuration / time.Duration(stats.TotalCalls)
	}

	return stats
}

// GetAllUsage 获取用户所有工具使用统计
func (t *UsageTracker) GetAllUsage(userID string) []UsageStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records, ok := t.records[userID]
	if !ok {
		return nil
	}

	// 按工具聚合
	toolMap := make(map[string]*UsageStats)
	for _, r := range records {
		stats, exists := toolMap[r.ToolName]
		if !exists {
			stats = &UsageStats{ToolName: r.ToolName}
			toolMap[r.ToolName] = stats
		}
		stats.TotalCalls++
		if r.Success {
			stats.SuccessCalls++
		} else {
			stats.FailedCalls++
		}
		stats.TotalDuration += r.Duration
		if r.Timestamp.After(stats.LastUsed) {
			stats.LastUsed = r.Timestamp
		}
	}

	var result []UsageStats
	for _, stats := range toolMap {
		if stats.TotalCalls > 0 {
			stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalCalls)
		}
		result = append(result, *stats)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCalls > result[j].TotalCalls
	})

	return result
}

// GetQuota 获取配额信息
func (t *UsageTracker) GetQuota(userID, toolName string) *Quota {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := userID + ":" + toolName
	if q, ok := t.quotas[key]; ok {
		qCopy := *q
		return &qCopy
	}
	return nil
}

// ListQuotas 列出用户所有配额
func (t *UsageTracker) ListQuotas(userID string) []Quota {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var quotas []Quota
	for key, q := range t.quotas {
		if strings.HasPrefix(key, userID+":") {
			quotas = append(quotas, *q)
		}
	}

	sort.Slice(quotas, func(i, j int) bool {
		return quotas[i].ToolName < quotas[j].ToolName
	})

	return quotas
}

// UsageStats 使用统计
type UsageStats struct {
	ToolName      string        `json:"tool_name"`
	TotalCalls    int           `json:"total_calls"`
	SuccessCalls  int           `json:"success_calls"`
	FailedCalls   int           `json:"failed_calls"`
	TotalDuration time.Duration `json:"total_duration"`
	AvgDuration   time.Duration `json:"avg_duration"`
	LastUsed      time.Time     `json:"last_used"`
}

// Format 格式化统计
func (s UsageStats) Format() string {
	successRate := float64(0)
	if s.TotalCalls > 0 {
		successRate = float64(s.SuccessCalls) / float64(s.TotalCalls) * 100
	}
	return fmt.Sprintf("  %s: %d calls (%.0f%% success, avg %v, last %s)",
		s.ToolName, s.TotalCalls, successRate, s.AvgDuration,
		s.LastUsed.Format("2006-01-02 15:04"))
}
