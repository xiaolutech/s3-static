package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const optimizerTriggerQueueSize = 256

type OptimizerTrigger interface {
	Trigger(ctx context.Context, key string) error
}

type HTTPOptimizerTrigger struct {
	endpoint string
	client   *http.Client
	keys     chan string
	mu       sync.Mutex
	pending  map[string]struct{}
}

func NewHTTPOptimizerTrigger(endpoint string, timeout time.Duration) *HTTPOptimizerTrigger {
	trigger := &HTTPOptimizerTrigger{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: timeout,
		},
		keys:    make(chan string, optimizerTriggerQueueSize),
		pending: make(map[string]struct{}),
	}
	go trigger.run()
	return trigger
}

func (t *HTTPOptimizerTrigger) Trigger(ctx context.Context, key string) error {
	if t == nil || t.endpoint == "" {
		return nil
	}
	if key == "" {
		return fmt.Errorf("optimizer trigger key is required")
	}

	t.mu.Lock()
	if _, ok := t.pending[key]; ok {
		t.mu.Unlock()
		return nil
	}
	if len(t.pending) >= cap(t.keys) {
		t.mu.Unlock()
		return fmt.Errorf("optimizer trigger queue is full")
	}
	t.pending[key] = struct{}{}
	t.mu.Unlock()

	select {
	case t.keys <- key:
		return nil
	default:
		t.finish(key)
		return fmt.Errorf("optimizer trigger queue is full")
	}
}

func (t *HTTPOptimizerTrigger) run() {
	for key := range t.keys {
		_ = t.post(context.Background(), key)
		t.finish(key)
	}
}

func (t *HTTPOptimizerTrigger) finish(key string) {
	t.mu.Lock()
	delete(t.pending, key)
	t.mu.Unlock()
}

func (t *HTTPOptimizerTrigger) post(ctx context.Context, key string) error {
	endpointURL, err := url.Parse(t.endpoint)
	if err != nil {
		return fmt.Errorf("parse optimizer trigger URL: %w", err)
	}
	values := endpointURL.Query()
	values.Set("key", key)
	endpointURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create optimizer trigger request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("post optimizer trigger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("optimizer trigger returned %s", resp.Status)
	}
	return nil
}
