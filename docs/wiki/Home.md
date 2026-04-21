# LuckyHarness Wiki

> 🍀 一个模块化的 Go AI Agent 框架，支持多平台消息网关、工具调用、RAG 知识库、定时任务等。

## 目录

- [项目概览](./Home) — 你在这里
- [架构总览](./Architecture) — 系统架构与模块关系
- [已接入模块](./Integrated-Modules) — 当前已接入并正常工作的功能
- [Bot 命令手册](./Bot-Commands) — Telegram Bot 可用命令
- [HTTP API 文档](./HTTP-API) — REST API 接口文档
- [Skill 系统](./Skill-System) — 88 个 Skill 的加载与调度
- [配置指南](./Configuration) — 配置文件与启动参数
- [开发路线图](./Roadmap) — 已完成与待开发功能

## 快速开始

```bash
# 构建
go build -o lh ./cmd/lh/

# 启动 Telegram Bot（同时启动 HTTP API :9090）
nohup ./lh msg-gateway start --platform telegram --token <BOT_TOKEN> &

# 交互式聊天
./lh chat --yolo

# 运行测试
go test ./...
```

## 版本

当前版本：**v0.36.0**

## 仓库

[github.com/yurika0211/luckyharness](https://github.com/yurika0211/luckyharness)