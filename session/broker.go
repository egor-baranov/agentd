package session

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Event struct {
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	Type      string         `json:"type"`
	Sequence  int64          `json:"sequence"`
	Data      map[string]any `json:"data,omitempty"`
	Time      time.Time      `json:"time"`
}

type Broker interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, sessionID string) (<-chan Event, func())
}

type MemoryBroker struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{subs: map[string]map[chan Event]struct{}{}}
}

func (b *MemoryBroker) Publish(_ context.Context, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[event.SessionID] {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (b *MemoryBroker) Subscribe(_ context.Context, sessionID string) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[sessionID] == nil {
		b.subs[sessionID] = map[chan Event]struct{}{}
	}
	b.subs[sessionID][ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[sessionID], ch)
		close(ch)
	}
}

type RedisBroker struct {
	client *redis.Client
	prefix string
}

func NewRedisBroker(client *redis.Client, prefix string) *RedisBroker {
	if prefix == "" {
		prefix = "agentd:sessions:"
	}
	return &RedisBroker{client: client, prefix: prefix}
}

func (b *RedisBroker) Publish(ctx context.Context, event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.prefix+event.SessionID, raw).Err()
}

func (b *RedisBroker) Subscribe(ctx context.Context, sessionID string) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	pubsub := b.client.Subscribe(ctx, b.prefix+sessionID)
	go func() {
		defer close(ch)
		for msg := range pubsub.Channel() {
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch, func() { _ = pubsub.Close() }
}
