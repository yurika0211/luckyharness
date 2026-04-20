package collab

import "errors"

var (
	// ErrInvalidMode 无效协作模式
	ErrInvalidMode = errors.New("invalid collaboration mode: must be pipeline, parallel, or debate")
	// ErrInvalidDebateRounds 无效辩论轮数
	ErrInvalidDebateRounds = errors.New("debate rounds must be at least 1")
	// ErrInvalidMaxConcurrent 无效最大并发数
	ErrInvalidMaxConcurrent = errors.New("max concurrent must be at least 1")
	// ErrInvalidAggregation 无效聚合策略
	ErrInvalidAggregation = errors.New("invalid aggregation strategy: must be concat, best, vote, merge, or summary")
	// ErrAgentNotFound Agent 不存在
	ErrAgentNotFound = errors.New("agent not found in registry")
	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskAlreadyCompleted 任务已完成
	ErrTaskAlreadyCompleted = errors.New("task already completed or cancelled")
	// ErrNoHandler 未配置任务处理器
	ErrNoHandler = errors.New("no task handler configured")
)