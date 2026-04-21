# Skill 系统

LuckyHarness 的 Skill 系统是框架的核心扩展机制，当前加载了 **88 个 Skill**。

## 架构设计

### 两层设计（v0.36.0）

```
┌─────────────────────────────────────────────┐
│           System Prompt (buildMessages)      │
│                                              │
│  ┌─────────────────────────────────────────┐ │
│  │  Skill 摘要层 (~8.7K tokens)            │ │
│  │  88 个 Skill 的 Trigger/Summary         │ │
│  │  → LLM 知道有哪些 Skill 可用            │ │
│  └─────────────────────────────────────────┘ │
│                                              │
│  ┌─────────────────────────────────────────┐ │
│  │  skill_read(name) 工具                   │ │
│  │  按需读取完整 SKILL.md                   │ │
│  │  → 只在需要时才消耗 token                │ │
│  └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

**为什么两层？**
- 全部 88 个 Skill 的完整 SKILL.md 太大（>200K tokens），无法全部注入
- 摘要层让 LLM 知道"有什么可用"，按需读取层提供"怎么用"
- 平衡了上下文长度和功能覆盖

### SkillLoader

- **Frontmatter 解析**：读取 YAML frontmatter 中的元数据
- **Tools 自动生成**：没有 `## Tools` section 的 Skill 自动生成 `run` tool
- **脚本执行**：通过 `SKILL_ARG_*` 环境变量传递参数给脚本
- **sanitizeName**：Skill 名称规范化

### extractSummary

解析 SKILL.md 中的以下 section 生成摘要：
- Trigger / 触发条件
- Workflow / 工作流
- Steps / 步骤
- 其他关键 section

当前 **84/88** 个 Skill 有摘要。

## Skill 目录

Skill 文件位于 `~/.luckyharness/skills/`，每个 Skill 是一个目录：

```
~/.luckyharness/skills/
├── web-search/
│   └── SKILL.md
├── video-tool/
│   ├── SKILL.md
│   └── scripts/
│       └── transcribe.sh
├── image-gen/
│   ├── SKILL.md
│   └── scripts/
│       └── generate_image.py
└── ...（共 88 个）
```

## Skill 分类

### 内容创作
- `web-search` — 联网搜索
- `summarize` — 内容摘要
- `rewrite` — 改写润色
- `content-writer` — 多平台内容生成
- `wechat-publisher` — 公众号发布
- `xiaohongshu-ops` — 小红书运营
- `zhihu-publisher` — 知乎发布
- `weibo-publisher` — 微博发布
- `toutiao-safe-publish` — 头条号发布
- `douyin-image-publisher` — 抖音图文

### 开发工具
- `software-development` — 软件开发
- `github` — GitHub 工作流
- `git-version-control` — Git 版本控制
- `mcp` — MCP 协议集成
- `devops` — DevOps 工具

### 数据与可视化
- `data-report` — 数据报告
- `data-science` — 数据科学
- `image-gen` — AI 生图/图表/卡片
- `mermaid-visualizer` — Mermaid 图表
- `excalidraw` / `excalidraw-diagram` — 手绘风格图表
- `architecture-diagram` — 架构图

### 媒体处理
- `video-tool` — 视频转译/字幕/抽帧
- `media` — 媒体总入口
- `ascii-art` / `ascii-video` — ASCII 艺术
- `p5js` — p5.js 创意编程
- `manim-video` — 数学动画

### 知识管理
- `obsidian-cli` — Obsidian CLI
- `obsidian-markdown` — Obsidian Markdown
- `obsidian-canvas-creator` — Canvas 创建
- `obsidian-bases` — Obsidian Bases
- `obsidian-vault-structure` — Vault 结构设计
- `note-taking` — 笔记
- `memory` / `memory-sync` — 记忆系统

### 运营管理
- `ops-manager` — 会议任务化
- `cfo` / `cfo-manager` — 财报分析
- `ceo-advisor` — 经营顾问
- `automation` — 自动化
- `cron` — 定时任务

### 其他
- `weather` — 天气查询
- `leisure` — 休闲生活
- `gaming` — 游戏服务器
- `smart-home` — 智能家居
- `red-teaming` — 红队测试
- `security-best-practices` — 安全最佳实践
- `creative-ideation` — 创意生成
- `songwriting-and-ai-music` — 音乐创作
- `bangumi-api` — Bangumi API
- ...等