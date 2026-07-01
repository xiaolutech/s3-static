package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type OptimizerTrigger interface {
	Trigger(ctx context.Context, key string) error
}

type HTTPOptimizerTrigger struct {
	endpoint string
	client   *http.Client
}

func NewHTTPOptimizerTrigger(endpoint string, timeout time.Duration) *HTTPOptimizerTrigger {
	return &HTTPOptimizerTrigger{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (t *HTTPOptimizerTrigger) Trigger(ctx context.Context, key string) error {
	if t == nil || t.endpoint == "" {
		return nil
	}

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
