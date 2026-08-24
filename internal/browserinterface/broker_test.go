package browserinterface

import (
	"context"
	"testing"
	"time"
)

func TestBrokerReportsFirstDispatch(t *testing.T) {
	broker := NewBroker()
	dispatched := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := broker.requestContextWithDispatch(
			context.Background(),
			"job-1",
			map[string]any{"kind": "chatgpt.prepare_direct"},
			"chatgpt",
			dispatched,
		)
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	var job *Job
	for job == nil && time.Now().Before(deadline) {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("job was not dispatched")
	}
	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("dispatch notification was not delivered")
	}

	broker.Complete(job.ID, "chatgpt", map[string]any{"ok": true})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
