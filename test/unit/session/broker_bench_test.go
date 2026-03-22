package session_test

import (
	"context"
	"testing"
	"time"

	"agentd/session"
)

func BenchmarkPromptDispatch(b *testing.B) {
	broker := session.NewMemoryBroker()
	ctx := context.Background()
	ch, cancel := broker.Subscribe(ctx, "sess-1")
	defer cancel()
	go func() {
		for range ch {
		}
	}()
	event := session.Event{SessionID: "sess-1", Type: "chunk", Sequence: 1, Time: time.Now()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.Sequence = int64(i)
		if err := broker.Publish(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}
