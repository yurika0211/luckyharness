package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGateway is a test implementation of Gateway.
type mockGateway struct {
	name     string
	running  bool
	startErr error
	stopErr  error
	sentMsgs []string
}

func (m *mockGateway) Name() string { return m.name }

func (m *mockGateway) Start(_ context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.running = true
	return nil
}

func (m *mockGateway) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running = false
	return nil
}

func (m *mockGateway) Send(_ context.Context, _ string, message string) error {
	m.sentMsgs = append(m.sentMsgs, message)
	return nil
}

func (m *mockGateway) SendWithReply(_ context.Context, chatID string, _ string, message string) error {
	m.sentMsgs = append(m.sentMsgs, message)
	return nil
}

func (m *mockGateway) IsRunning() bool { return m.running }

func TestNewGatewayManager(t *testing.T) {
	gm := NewGatewayManager()
	assert.NotNil(t, gm)
	assert.Empty(t, gm.List())
	assert.False(t, gm.IsRunning())
}

func TestRegister(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}

	err := gm.Register(gw)
	require.NoError(t, err)

	assert.Contains(t, gm.List(), "test")

	gw2, ok := gm.Get("test")
	assert.True(t, ok)
	assert.Equal(t, gw, gw2)
}

func TestRegisterDuplicate(t *testing.T) {
	gm := NewGatewayManager()
	gw1 := &mockGateway{name: "test"}
	gw2 := &mockGateway{name: "test"}

	err := gm.Register(gw1)
	require.NoError(t, err)

	err = gm.Register(gw2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestUnregister(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}

	err := gm.Register(gw)
	require.NoError(t, err)

	err = gm.Unregister("test")
	require.NoError(t, err)

	_, ok := gm.Get("test")
	assert.False(t, ok)
}

func TestUnregisterNotFound(t *testing.T) {
	gm := NewGatewayManager()
	err := gm.Unregister("nonexistent")
	assert.Error(t, err)
}

func TestUnregisterRunningGateway(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}

	require.NoError(t, gm.Register(gw))
	require.NoError(t, gm.Start(context.Background(), "test"))
	assert.True(t, gw.IsRunning())

	require.NoError(t, gm.Unregister("test"))
	assert.False(t, gw.IsRunning())
}

func TestStart(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}
	require.NoError(t, gm.Register(gw))

	err := gm.Start(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, gw.IsRunning())
}

func TestStartNotFound(t *testing.T) {
	gm := NewGatewayManager()
	err := gm.Start(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestStartError(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test", startErr: assert.AnError}
	require.NoError(t, gm.Register(gw))

	err := gm.Start(context.Background(), "test")
	assert.Error(t, err)
}

func TestStartAll(t *testing.T) {
	gm := NewGatewayManager()
	gw1 := &mockGateway{name: "gw1"}
	gw2 := &mockGateway{name: "gw2"}

	require.NoError(t, gm.Register(gw1))
	require.NoError(t, gm.Register(gw2))

	err := gm.StartAll(context.Background())
	require.NoError(t, err)

	assert.True(t, gw1.IsRunning())
	assert.True(t, gw2.IsRunning())
	assert.True(t, gm.IsRunning())
}

func TestStop(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}
	require.NoError(t, gm.Register(gw))
	require.NoError(t, gm.Start(context.Background(), "test"))

	err := gm.Stop("test")
	require.NoError(t, err)
	assert.False(t, gw.IsRunning())
}

func TestStopNotFound(t *testing.T) {
	gm := NewGatewayManager()
	err := gm.Stop("nonexistent")
	assert.Error(t, err)
}

func TestStopAll(t *testing.T) {
	gm := NewGatewayManager()
	gw1 := &mockGateway{name: "gw1"}
	gw2 := &mockGateway{name: "gw2"}

	require.NoError(t, gm.Register(gw1))
	require.NoError(t, gm.Register(gw2))
	require.NoError(t, gm.StartAll(context.Background()))

	err := gm.StopAll()
	require.NoError(t, err)
	assert.False(t, gw1.IsRunning())
	assert.False(t, gw2.IsRunning())
	assert.False(t, gm.IsRunning())
}

func TestOnMessage(t *testing.T) {
	gm := NewGatewayManager()

	var received *Message
	gm.OnMessage(func(_ context.Context, msg *Message) error {
		received = msg
		return nil
	})

	msg := &Message{
		ID:   "1",
		Text: "hello",
		Chat: Chat{ID: "chat1", Type: ChatPrivate},
	}

	err := gm.handleMessage(context.Background(), "test", msg)
	require.NoError(t, err)
	assert.Equal(t, "hello", received.Text)
}

func TestStats(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}
	require.NoError(t, gm.Register(gw))

	gm.RecordSent("test")
	gm.RecordSent("test")
	gm.RecordError("test")

	stats, ok := gm.Stats("test")
	assert.True(t, ok)
	assert.Equal(t, int64(2), stats.MessagesSent)
	assert.Equal(t, int64(1), stats.Errors)
}

func TestStatsNotFound(t *testing.T) {
	gm := NewGatewayManager()
	_, ok := gm.Stats("nonexistent")
	assert.False(t, ok)
}

func TestAllStats(t *testing.T) {
	gm := NewGatewayManager()
	gw1 := &mockGateway{name: "gw1"}
	gw2 := &mockGateway{name: "gw2"}
	require.NoError(t, gm.Register(gw1))
	require.NoError(t, gm.Register(gw2))

	gm.RecordSent("gw1")
	gm.RecordSent("gw2")
	gm.RecordSent("gw2")

	allStats := gm.AllStats()
	assert.Len(t, allStats, 2)
	assert.Equal(t, int64(1), allStats["gw1"].MessagesSent)
	assert.Equal(t, int64(2), allStats["gw2"].MessagesSent)
}

func TestStatus(t *testing.T) {
	gm := NewGatewayManager()
	gw1 := &mockGateway{name: "gw1"}
	gw2 := &mockGateway{name: "gw2"}
	require.NoError(t, gm.Register(gw1))
	require.NoError(t, gm.Register(gw2))
	require.NoError(t, gm.Start(context.Background(), "gw1"))

	statuses := gm.Status()
	assert.Len(t, statuses, 2)

	// Find gw1 status
	var gw1Status *GatewayStatus
	for i := range statuses {
		if statuses[i].Name == "gw1" {
			gw1Status = &statuses[i]
		}
	}
	require.NotNil(t, gw1Status)
	assert.True(t, gw1Status.Running)
}

func TestHandleMessageNoHandler(t *testing.T) {
	gm := NewGatewayManager()
	msg := &Message{ID: "1", Text: "hello"}

	// Should not panic when no handler is set
	err := gm.handleMessage(context.Background(), "test", msg)
	assert.NoError(t, err)
}

func TestHandleMessageRecordsReceived(t *testing.T) {
	gm := NewGatewayManager()
	gw := &mockGateway{name: "test"}
	require.NoError(t, gm.Register(gw))

	gm.OnMessage(func(_ context.Context, msg *Message) error { return nil })

	msg := &Message{ID: "1", Text: "hello"}
	err := gm.handleMessage(context.Background(), "test", msg)
	require.NoError(t, err)

	stats, ok := gm.Stats("test")
	assert.True(t, ok)
	assert.Equal(t, int64(1), stats.MessagesReceived)
}