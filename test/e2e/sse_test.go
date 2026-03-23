package e2e_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentd/api"
	"agentd/observer"
	"agentd/session"
)

func TestSessionEventsSSE(t *testing.T) {
	broker := session.NewMemoryBroker()
	srv := httptest.NewServer((&api.Server{Metrics: observer.NewMetrics(), Broker: broker}).Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/sessions/sess-1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("sse request: %v", err)
	}
	defer resp.Body.Close()

	go func() {
		_ = broker.Publish(context.Background(), session.Event{SessionID: "sess-1", Type: "chunk", Sequence: 1, Time: time.Now()})
	}()
	reader := bufio.NewReader(resp.Body)
	for i := 0; i < 8; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
		if strings.HasPrefix(line, "event: chunk") {
			return
		}
	}
	t.Fatal("expected SSE event")
}
