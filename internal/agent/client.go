package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.ServerURL, "/"),
		token:   cfg.Token,
		http: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	}
}

func (c *Client) SendHeartbeat(ctx context.Context, payload heartbeatPayload) error {
	return c.post(ctx, "/api/agent/heartbeat", payload)
}

func (c *Client) SendMetrics(ctx context.Context, payload metricPayload) error {
	return c.post(ctx, "/api/agent/metrics", payload)
}

func (c *Client) SendDevices(ctx context.Context, payload devicePayload) error {
	return c.post(ctx, "/api/agent/devices", payload)
}

func (c *Client) SendHardware(ctx context.Context, payload hardwarePayload) error {
	return c.post(ctx, "/api/agent/hardware", payload)
}

func (c *Client) SendTemperatures(ctx context.Context, payload temperaturePayload) error {
	return c.post(ctx, "/api/agent/temperatures", payload)
}

func (c *Client) PollCommand(ctx context.Context) (*remoteCommand, error) {
	var response commandPollResponse
	if err := c.postDecode(ctx, "/api/agent/commands/poll", map[string]any{}, &response); err != nil {
		return nil, err
	}
	return response.Command, nil
}

func (c *Client) SendCommandResult(ctx context.Context, commandID int64, payload commandResultPayload) error {
	return c.post(ctx, fmt.Sprintf("/api/agent/commands/%d/result", commandID), payload)
}

func (c *Client) post(ctx context.Context, path string, payload any) error {
	return c.postDecode(ctx, path, payload, nil)
}

func (c *Client) postDecode(ctx context.Context, path string, payload any, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "InfoBHMonitorAgent/"+Version)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return err
		}
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
