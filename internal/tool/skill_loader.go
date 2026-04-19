package tool

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SkillLoader 从 SKILL.md 文件加载工具定义
type SkillLoader struct {
	skillsDir string // skills 目录路径
}

// NewSkillLoader 创建 Skill 加载器
func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{skillsDir: skillsDir}
}

// SkillInfo Skill 元信息
type SkillInfo struct {
	Name         string
	Description  string
	Dir          string
	Tools        []SkillToolDef
	Available    bool
}

// SkillToolDef Skill 中定义的工具
type SkillToolDef struct {
	Name        string
	Description string
	Parameters  map[string]Param
	Handler     func(args map[string]any) (string, error) // 需要外部注入
}

// LoadAll 加载所有 Skill
func (sl *SkillLoader) LoadAll() ([]*SkillInfo, error) {
	entries, err := os.ReadDir(sl.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var skills []*SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(sl.skillsDir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")

		info, err := sl.Load(skillFile)
		if err != nil {
			continue // 跳过无法加载的
		}
		info.Dir = skillDir
		skills = append(skills, info)
	}

	return skills, nil
}

// Load 加载单个 SKILL.md
func (sl *SkillLoader) Load(path string) (*SkillInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill: %w", err)
	}

	content := string(data)
	info := &SkillInfo{
		Available: true,
	}

	// 解析 Skill 名称（从目录名或标题）
	dir := filepath.Dir(path)
	info.Name = filepath.Base(dir)

	// 解析标题（第一个 # 标题）
	if match := regexp.MustCompile(`(?m)^#\s+(.+)$`).FindStringSubmatch(content); len(match) > 1 {
		info.Name = strings.TrimSpace(match[1])
	}

	// 解析描述（标题后的第一段非空文本）
	lines := strings.Split(content, "\n")
	inHeader := true
	for _, line := range lines {
		if inHeader {
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			inHeader = false
		}
		if strings.TrimSpace(line) != "" {
			info.Description = strings.TrimSpace(line)
			break
		}
	}

	// 解析工具定义
	// 格式: ## Tools 或 ## 可用工具 后跟工具列表
	info.Tools = sl.parseToolDefs(content)

	return info, nil
}

// parseToolDefs 从 SKILL.md 内容解析工具定义
func (sl *SkillLoader) parseToolDefs(content string) []SkillToolDef {
	var tools []SkillToolDef

	// 查找工具区域
	toolSection := sl.extractSection(content, "Tools")
	if toolSection == "" {
		toolSection = sl.extractSection(content, "可用工具")
	}
	if toolSection == "" {
		toolSection = sl.extractSection(content, "工具")
	}

	if toolSection == "" {
		return tools
	}

	// 解析工具条目
	// 格式: - `tool_name`: description
	// 或: - **tool_name**: description
	scanner := bufio.NewScanner(strings.NewReader(toolSection))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// 匹配列表项
		if !strings.HasPrefix(line, "-") {
			continue
		}
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)

		name, desc := parseToolEntry(line)
		if name == "" {
			continue
		}

		tools = append(tools, SkillToolDef{
			Name:        name,
			Description: desc,
			Parameters:  map[string]Param{}, // 参数从 SKILL.md 解析较复杂，后续版本完善
		})
	}

	return tools
}

// extractSection 提取 Markdown section
func (sl *SkillLoader) extractSection(content, title string) string {
	// 查找 ## title
	sectionStart := -1
	sectionEnd := -1

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)
			if strings.EqualFold(heading, title) {
				sectionStart = i + 1
				continue
			}
			// 遇到下一个 ## 结束
			if sectionStart >= 0 && sectionEnd < 0 {
				sectionEnd = i
			}
		}
	}

	if sectionStart < 0 {
		return ""
	}
	if sectionEnd < 0 {
		sectionEnd = len(lines)
	}

	return strings.Join(lines[sectionStart:sectionEnd], "\n")
}

// parseToolEntry 解析工具条目
func parseToolEntry(line string) (name, desc string) {
	// 格式1: `tool_name`: description
	re1 := regexp.MustCompile("`([^`]+)`\\s*[:：]\\s*(.+)")
	if match := re1.FindStringSubmatch(line); len(match) > 2 {
		return match[1], match[2]
	}

	// 格式2: **tool_name**: description
	re2 := regexp.MustCompile(`\*\*([^*]+)\*\*\s*[:：]\s*(.+)`)
	if match := re2.FindStringSubmatch(line); len(match) > 2 {
		return match[1], match[2]
	}

	// 格式3: tool_name - description
	re3 := regexp.MustCompile(`^(\w[\w_-]*)\s*[-—–]\s*(.+)`)
	if match := re3.FindStringSubmatch(line); len(match) > 2 {
		return match[1], match[2]
	}

	return "", ""
}

// RegisterSkillTools 将 Skill 工具注册到 Registry
// handler 为 Skill 工具的通用处理器（通常调用脚本或子进程）
func RegisterSkillTools(r *Registry, skills []*SkillInfo, handler func(toolName string, skillDir string) func(args map[string]any) (string, error)) {
	for _, skill := range skills {
		for _, toolDef := range skill.Tools {
			tool := &Tool{
				Name:        fmt.Sprintf("skill_%s_%s", skill.Name, toolDef.Name),
				Description: toolDef.Description,
				Parameters:  toolDef.Parameters,
				Category:    CatSkill,
				Source:      skill.Name,
				Permission:  PermApprove, // Skill 工具默认需要审批
				Enabled:     true,
			}
			if handler != nil {
				tool.Handler = handler(toolDef.Name, skill.Dir)
			} else {
				// 默认处理器：返回占位信息
				tool.Handler = func(args map[string]any) (string, error) {
					return fmt.Sprintf("Skill tool '%s' from '%s' — handler not implemented", toolDef.Name, skill.Name), nil
				}
			}
			r.Register(tool)
		}
	}
}
