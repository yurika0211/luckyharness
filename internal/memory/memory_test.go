package memory

import (
	"os"
	"testing"
	"time"
)

func TestSaveAndSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Save("user prefers Chinese", "preference"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save("project uses Go", "context"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	results := s.Search("Chinese")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	results = s.Search("Go")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	for i := 0; i < 10; i++ {
		s.Save("memory item", "test")
	}

	recent := s.Recent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3, got %d", len(recent))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore1: %v", err)
	}
	s1.Save("persistent memory", "test")

	// Reload
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore2: %v", err)
	}
	results := s2.Search("persistent")
	if len(results) != 1 {
		t.Errorf("expected 1 persistent result, got %d", len(results))
	}
}

// --- v0.4.0 新测试 ---

func TestThreeTierSave(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 短期记忆
	if err := s.SaveShortTerm("current task: fix bug #42", "task"); err != nil {
		t.Fatalf("SaveShortTerm: %v", err)
	}

	// 中期记忆（默认）
	if err := s.Save("user likes dark mode", "preference"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 长期记忆
	if err := s.SaveLongTerm("project name: LuckyHarness", "identity"); err != nil {
		t.Fatalf("SaveLongTerm: %v", err)
	}

	stats := s.Stats()
	if stats[TierShort] != 1 {
		t.Errorf("expected 1 short, got %d", stats[TierShort])
	}
	if stats[TierMedium] != 1 {
		t.Errorf("expected 1 medium, got %d", stats[TierMedium])
	}
	if stats[TierLong] != 1 {
		t.Errorf("expected 1 long, got %d", stats[TierLong])
	}
}

func TestSaveWithTier(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.SaveWithTier("high importance note", "critical", TierLong, 0.95); err != nil {
		t.Fatalf("SaveWithTier: %v", err)
	}

	results := s.Search("high importance")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Tier != TierLong {
		t.Errorf("expected TierLong, got %v", results[0].Tier)
	}
	if results[0].Importance < 0.9 {
		t.Errorf("expected importance >= 0.9, got %f", results[0].Importance)
	}
}

func TestByTier(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.SaveShortTerm("short1", "test")
	s.SaveShortTerm("short2", "test")
	s.Save("medium1", "test")
	s.SaveLongTerm("long1", "test")

	shorts := s.ByTier(TierShort)
	if len(shorts) != 2 {
		t.Errorf("expected 2 short, got %d", len(shorts))
	}

	mediums := s.ByTier(TierMedium)
	if len(mediums) != 1 {
		t.Errorf("expected 1 medium, got %d", len(mediums))
	}

	longs := s.ByTier(TierLong)
	if len(longs) != 1 {
		t.Errorf("expected 1 long, got %d", len(longs))
	}
}

func TestByCategory(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Save("pref1", "preference")
	s.Save("pref2", "preference")
	s.Save("ctx1", "context")

	prefs := s.ByCategory("preference")
	if len(prefs) != 2 {
		t.Errorf("expected 2 preference, got %d", len(prefs))
	}

	ctxs := s.ByCategory("context")
	if len(ctxs) != 1 {
		t.Errorf("expected 1 context, got %d", len(ctxs))
	}
}

func TestPromote(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.SaveShortTerm("temp note", "test")

	// 找到 ID
	shorts := s.ByTier(TierShort)
	if len(shorts) != 1 {
		t.Fatalf("expected 1 short, got %d", len(shorts))
	}
	id := shorts[0].ID

	// 提升到中期
	if err := s.Promote(id); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// 验证层级变化
	entry, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Tier != TierMedium {
		t.Errorf("expected TierMedium after promote, got %v", entry.Tier)
	}

	// 再提升到长期
	if err := s.Promote(id); err != nil {
		t.Fatalf("Promote2: %v", err)
	}
	entry, _ = s.Get(id)
	if entry.Tier != TierLong {
		t.Errorf("expected TierLong after second promote, got %v", entry.Tier)
	}

	// 长期再提升应该无操作
	if err := s.Promote(id); err != nil {
		t.Fatalf("Promote3: %v", err)
	}
	entry, _ = s.Get(id)
	if entry.Tier != TierLong {
		t.Errorf("expected still TierLong, got %v", entry.Tier)
	}
}

func TestDecay(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 保存一条短期记忆（低重要性）
	s.SaveWithTier("unimportant temp", "test", TierShort, 0.1)
	// 保存一条长期记忆
	s.SaveLongTerm("important core", "test")

	// 短期记忆在创建时权重 = 0.1
	// 衰减阈值 0.05 应该不会删除刚创建的
	deleted := s.Decay(0.05)
	if deleted != 0 {
		t.Errorf("expected 0 deleted (too recent), got %d", deleted)
	}

	// 手动修改创建时间模拟老化
	shorts := s.ByTier(TierShort)
	if len(shorts) != 1 {
		t.Fatalf("expected 1 short, got %d", len(shorts))
	}

	// 直接操作 entry 模拟时间流逝
	s.mu.Lock()
	for _, e := range s.entries {
		if e.Tier == TierShort {
			// 设为 10 小时前（超过短期半衰期 1h）
			e.CreatedAt = time.Now().Add(-10 * time.Hour)
		}
	}
	s.mu.Unlock()

	// 现在衰减应该删除短期记忆
	deleted = s.Decay(0.05)
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// 长期记忆不应被衰减
	stats := s.Stats()
	if stats[TierLong] != 1 {
		t.Errorf("expected 1 long (not decayed), got %d", stats[TierLong])
	}
}

func TestSummarize(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Save("user likes Go", "preference")
	s.Save("user likes Rust", "preference")
	s.Save("user likes Vim", "preference")

	if s.Count() != 3 {
		t.Errorf("expected 3 entries, got %d", s.Count())
	}

	// 收集 ID
	prefs := s.ByCategory("preference")
	ids := make([]string, len(prefs))
	for i, p := range prefs {
		ids[i] = p.ID
	}

	// 压缩为摘要
	err = s.Summarize(ids, "user prefers Go, Rust, and Vim", "preference")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	// 应该只剩 1 条摘要
	if s.Count() != 1 {
		t.Errorf("expected 1 entry after summarize, got %d", s.Count())
	}

	// 摘要条目应该有 SummaryOf
	all := s.ByCategory("preference")
	if len(all) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(all))
	}
	if len(all[0].SummaryOf) != 3 {
		t.Errorf("expected SummaryOf 3, got %d", len(all[0].SummaryOf))
	}
}

func TestSearchWeighted(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 高重要性长期记忆
	s.SaveWithTier("critical: API key rotation needed", "security", TierLong, 0.95)
	// 低重要性短期记忆
	s.SaveWithTier("todo: fix typo", "task", TierShort, 0.1)

	results := s.Search("fix")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'fix', got %d", len(results))
	}

	// 搜索 "API" 应该找到高重要性条目
	results = s.Search("API")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'API', got %d", len(results))
	}
	if results[0].Importance < 0.9 {
		t.Errorf("expected high importance result, got %f", results[0].Importance)
	}
}

func TestAccessCount(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Save("frequently accessed memory", "test")

	// 多次搜索同一内容
	for i := 0; i < 5; i++ {
		s.Search("frequently")
	}

	// 验证访问计数
	results := s.Search("frequently")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].AccessCount < 5 {
		t.Errorf("expected access count >= 5, got %d", results[0].AccessCount)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Save("to be deleted", "test")
	if s.Count() != 1 {
		t.Errorf("expected 1, got %d", s.Count())
	}

	results := s.Search("to be deleted")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if err := s.Delete(results[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("expected 0 after delete, got %d", s.Count())
	}

	// 删除不存在的
	if err := s.Delete("nonexistent"); err == nil {
		t.Error("expected error for nonexistent delete")
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Save("test content", "test")
	all := s.ByCategory("test")
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}

	entry, err := s.Get(all[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Content != "test content" {
		t.Errorf("expected 'test content', got '%s'", entry.Content)
	}

	// 不存在的
	_, err = s.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent get")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.SaveShortTerm("s1", "test")
	s.SaveShortTerm("s2", "test")
	s.Save("m1", "test")
	s.SaveLongTerm("l1", "test")

	stats := s.Stats()
	total := stats[TierShort] + stats[TierMedium] + stats[TierLong]
	if total != 4 {
		t.Errorf("expected total 4, got %d", total)
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier    Tier
		expect  string
	}{
		{TierShort, "short"},
		{TierMedium, "medium"},
		{TierLong, "long"},
		{Tier(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.expect {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.expect)
		}
	}
}

func TestEntryWeight(t *testing.T) {
	now := time.Now()

	// 新创建的高重要性长期记忆
	e1 := &Entry{
		Tier:       TierLong,
		Importance:  0.9,
		CreatedAt:  now,
		AccessCount: 0,
	}
	w1 := e1.Weight(now)
	if w1 < 0.8 {
		t.Errorf("high importance long-term weight too low: %f", w1)
	}

	// 很旧的低重要性短期记忆
	e2 := &Entry{
		Tier:       TierShort,
		Importance:  0.1,
		CreatedAt:  now.Add(-10 * time.Hour),
		AccessCount: 0,
	}
	w2 := e2.Weight(now)
	if w2 >= w1 {
		t.Errorf("old short-term weight should be less than new long-term: %f >= %f", w2, w1)
	}

	// 高访问次数加成
	e3 := &Entry{
		Tier:       TierMedium,
		Importance:  0.5,
		CreatedAt:  now,
		AccessCount: 10,
	}
	w3 := e3.Weight(now)
	if w3 <= 0.5 {
		t.Errorf("frequently accessed weight should have bonus: %f", w3)
	}
}

func TestMigrateOldFormat(t *testing.T) {
	dir := t.TempDir()

	// 写入旧格式文件
	oldData := "old memory line 1\nold memory line 2\nold memory line 3\n"
	oldPath := dir + "/memory.txt"
	if err := os.WriteFile(oldPath, []byte(oldData), 0600); err != nil {
		t.Fatalf("write old format: %v", err)
	}

	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore with old format: %v", err)
	}

	// 应该迁移了 3 条
	if s.Count() != 3 {
		t.Errorf("expected 3 migrated entries, got %d", s.Count())
	}

	// 验证迁移后的层级
	stats := s.Stats()
	if stats[TierMedium] != 3 {
		t.Errorf("expected 3 medium (migrated), got %d", stats[TierMedium])
	}

	// 搜索应该能找到
	results := s.Search("old memory")
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestPersistenceWithNewFields(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore1: %v", err)
	}

	s1.SaveWithTier("tier test", "test", TierLong, 0.8)
	s1.SaveShortTerm("short test", "test")

	// Reload
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore2: %v", err)
	}

	stats := s2.Stats()
	if stats[TierLong] != 1 {
		t.Errorf("expected 1 long after reload, got %d", stats[TierLong])
	}
	if stats[TierShort] != 1 {
		t.Errorf("expected 1 short after reload, got %d", stats[TierShort])
	}

	longs := s2.ByTier(TierLong)
	if len(longs) != 1 {
		t.Fatalf("expected 1 long, got %d", len(longs))
	}
	if longs[0].Importance < 0.7 {
		t.Errorf("expected importance preserved, got %f", longs[0].Importance)
	}
}
