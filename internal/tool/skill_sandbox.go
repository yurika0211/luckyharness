package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SandboxConfig holds configuration for the skill execution sandbox.
type SandboxConfig struct {
	MaxExecutionTime time.Duration // maximum time a tool can run
	MaxOutputSize    int           // maximum output size in bytes
	EnableAuditLog   bool          // whether to record audit logs
}

// DefaultSandboxConfig returns the default sandbox configuration.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		MaxExecutionTime: 30 * time.Second,
		MaxOutputSize:    65536, // 64KB
		EnableAuditLog:   true,
	}
}

// AuditEntry records a single skill tool invocation for audit purposes.
type AuditEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	SkillName  string                 `json:"skill_name"`
	Args       map[string]any         `json:"args,omitempty"`
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Success    bool                   `json:"success"`
	OutputSize int                    `json:"output_size"`
	Panic      bool                   `json:"panic,omitempty"`
}

// SandboxError is a structured error returned from sandbox execution.
type SandboxError struct {
	ToolName string `json:"tool_name"`
	Message  string `json:"message"`
	Panic    bool   `json:"panic,omitempty"`
	Timeout  bool   `json:"timeout,omitempty"`
	Oversize bool   `json:"oversize,omitempty"`
}

func (e *SandboxError) Error() string {
	switch {
	case e.Timeout:
		return fmt.Sprintf("sandbox: tool %s timed out", e.ToolName)
	case e.Panic:
		return fmt.Sprintf("sandbox: tool %s panicked: %s", e.ToolName, e.Message)
	case e.Oversize:
		return fmt.Sprintf("sandbox: tool %s output exceeded size limit", e.ToolName)
	default:
		return fmt.Sprintf("sandbox: tool %s error: %s", e.ToolName, e.Message)
	}
}

// SkillSandbox provides an isolated execution environment for skill tools.
type SkillSandbox struct {
	mu       sync.RWMutex
	config   SandboxConfig
	auditLog []AuditEntry
	registry *Registry
}

// NewSkillSandbox creates a new skill execution sandbox.
func NewSkillSandbox(registry *Registry, config SandboxConfig) *SkillSandbox {
	return &SkillSandbox{
		config:   config,
		auditLog: make([]AuditEntry, 0),
		registry: registry,
	}
}

// Execute runs a tool in the sandbox with timeout, resource limits, and error recovery.
func (sb *SkillSandbox) Execute(toolName string, args map[string]any) (string, error) {
	start := time.Now()

	// Look up the tool
	t, ok := sb.registry.Get(toolName)
	if !ok {
		return "", &SandboxError{ToolName: toolName, Message: "tool not found"}
	}
	if !t.Enabled {
		return "", &SandboxError{ToolName: toolName, Message: "tool disabled"}
	}

	// Determine skill name from source
	skillName := t.Source

	// Execute with timeout and panic recovery
	result, execErr, panicked := sb.executeWithRecovery(toolName, t.Handler, args)
	duration := time.Since(start)

	// Check output size
	outputSize := len(result)
	if sb.config.MaxOutputSize > 0 && outputSize > sb.config.MaxOutputSize {
		result = result[:sb.config.MaxOutputSize] + "\n... (truncated by sandbox)"
		if execErr == nil {
			execErr = &SandboxError{ToolName: toolName, Oversize: true, Message: "output exceeded size limit"}
		}
	}

	// Build audit entry
	entry := AuditEntry{
		Timestamp:  start,
		ToolName:   toolName,
		SkillName:  skillName,
		Duration:   duration,
		Success:    execErr == nil && !panicked,
		OutputSize: outputSize,
		Panic:      panicked,
	}

	if panicked {
		entry.Error = result
		if execErr == nil {
			execErr = &SandboxError{ToolName: toolName, Panic: true, Message: result}
		}
	} else if execErr != nil {
		entry.Error = execErr.Error()
	} else {
		entry.Result = truncateStr(result, 1000) // truncate stored result
	}

	// Record args (truncate large values)
	if sb.config.EnableAuditLog {
		entry.Args = truncateArgs(args, 200)
		sb.recordAudit(entry)
	}

	return result, execErr
}

// executeWithRecovery runs the handler with panic recovery and timeout.
func (sb *SkillSandbox) executeWithRecovery(toolName string, handler func(args map[string]any) (string, error), args map[string]any) (result string, err error, panicked bool) {
	type outcome struct {
		result   string
		err      error
		panicked bool
	}

	done := make(chan outcome, 1)

	go func() {
		// Panic recovery
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{
					result:   fmt.Sprintf("%v", r),
					panicked: true,
				}
			}
		}()

		res, execErr := handler(args)
		done <- outcome{result: res, err: execErr}
	}()

	timeout := sb.config.MaxExecutionTime
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case o := <-done:
		return o.result, o.err, o.panicked
	case <-time.After(timeout):
		return "", &SandboxError{ToolName: toolName, Timeout: true, Message: "execution timed out"}, false
	}
}

// recordAudit appends an audit entry (thread-safe).
func (sb *SkillSandbox) recordAudit(entry AuditEntry) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.auditLog = append(sb.auditLog, entry)

	// Keep at most 10000 entries
	if len(sb.auditLog) > 10000 {
		sb.auditLog = sb.auditLog[len(sb.auditLog)-10000:]
	}
}

// AuditLog returns a copy of the audit log.
func (sb *SkillSandbox) AuditLog() []AuditEntry {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	result := make([]AuditEntry, len(sb.auditLog))
	copy(result, sb.auditLog)
	return result
}

// AuditLogForTool returns audit entries for a specific tool.
func (sb *SkillSandbox) AuditLogForTool(toolName string) []AuditEntry {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range sb.auditLog {
		if entry.ToolName == toolName {
			result = append(result, entry)
		}
	}
	return result
}

// AuditLogForSkill returns audit entries for a specific skill.
func (sb *SkillSandbox) AuditLogForSkill(skillName string) []AuditEntry {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range sb.auditLog {
		if entry.SkillName == skillName {
			result = append(result, entry)
		}
	}
	return result
}

// ClearAuditLog clears the audit log.
func (sb *SkillSandbox) ClearAuditLog() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.auditLog = sb.auditLog[:0]
}

// AuditStats returns summary statistics from the audit log.
func (sb *SkillSandbox) AuditStats() AuditStats {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	stats := AuditStats{}
	for _, entry := range sb.auditLog {
		stats.TotalInvocations++
		if entry.Success {
			stats.SuccessfulInvocations++
		} else {
			stats.FailedInvocations++
		}
		if entry.Panic {
			stats.PanicCount++
		}
		stats.TotalDuration += entry.Duration
	}
	if stats.TotalInvocations > 0 {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalInvocations)
	}
	return stats
}

// AuditStats holds summary audit statistics.
type AuditStats struct {
	TotalInvocations    int           `json:"total_invocations"`
	SuccessfulInvocations int         `json:"successful_invocations"`
	FailedInvocations   int           `json:"failed_invocations"`
	PanicCount          int           `json:"panic_count"`
	TotalDuration       time.Duration `json:"total_duration"`
	AvgDuration         time.Duration `json:"avg_duration"`
}

// ExecuteWithContext runs a tool in the sandbox with a context for cancellation.
func (sb *SkillSandbox) ExecuteWithContext(ctx context.Context, toolName string, args map[string]any) (string, error) {
	start := time.Now()

	t, ok := sb.registry.Get(toolName)
	if !ok {
		return "", &SandboxError{ToolName: toolName, Message: "tool not found"}
	}
	if !t.Enabled {
		return "", &SandboxError{ToolName: toolName, Message: "tool disabled"}
	}

	skillName := t.Source

	type outcome struct {
		result   string
		err      error
		panicked bool
	}

	done := make(chan outcome, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{result: fmt.Sprintf("%v", r), panicked: true}
			}
		}()

		res, execErr := t.Handler(args)
		done <- outcome{result: res, err: execErr}
	}()

	timeout := sb.config.MaxExecutionTime
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var o outcome
	select {
	case o = <-done:
	case <-ctx.Done():
		return "", &SandboxError{ToolName: toolName, Timeout: true, Message: "context cancelled"}
	case <-time.After(timeout):
		return "", &SandboxError{ToolName: toolName, Timeout: true, Message: "execution timed out"}
	}

	duration := time.Since(start)
	result := o.result
	outputSize := len(result)

	if sb.config.MaxOutputSize > 0 && outputSize > sb.config.MaxOutputSize {
		result = result[:sb.config.MaxOutputSize] + "\n... (truncated by sandbox)"
	}

	entry := AuditEntry{
		Timestamp:  start,
		ToolName:   toolName,
		SkillName:  skillName,
		Duration:   duration,
		Success:    o.err == nil && !o.panicked,
		OutputSize: outputSize,
		Panic:      o.panicked,
	}

	if o.panicked {
		entry.Error = o.result
	} else if o.err != nil {
		entry.Error = o.err.Error()
	} else {
		entry.Result = truncateStr(result, 1000)
	}

	if sb.config.EnableAuditLog {
		entry.Args = truncateArgs(args, 200)
		sb.recordAudit(entry)
	}

	if o.panicked {
		return result, &SandboxError{ToolName: toolName, Panic: true, Message: o.result}
	}

	return result, o.err
}

// truncateArgs truncates argument values for audit storage.
func truncateArgs(args map[string]any, maxLen int) map[string]any {
	if args == nil {
		return nil
	}
	result := make(map[string]any, len(args))
	for k, v := range args {
		if s, ok := v.(string); ok && len(s) > maxLen {
			result[k] = s[:maxLen] + "..."
		} else {
			result[k] = v
		}
	}
	return result
}

// FormatAuditEntry formats an audit entry as JSON string.
func FormatAuditEntry(entry AuditEntry) string {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Sprintf("{\"error\":\"%s\"}", err.Error())
	}
	return string(data)
}