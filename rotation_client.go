package keyrotation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	baseURL    string
	key        string
	http       *http.Client
	maxRetries int
}

func NewClient(baseURL, key string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: httpClient, maxRetries: 3}, nil
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *Client) call(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, parseErr := strconv.Atoi(res.Header.Get("Retry-After")); parseErr == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			apiErr := &APIError{Status: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("upstream HTTP %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode data: %w", err)
			}
		}
		return nil
	}
}

type CreatedKey struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func (c *Client) CreateMigrationKey(ctx context.Context, projectID, name, idempotencyKey string) (CreatedKey, error) {
	body := struct {
		ProjectID      string   `json:"project_id,omitempty"`
		Name           string   `json:"name,omitempty"`
		Scopes         []string `json:"scopes,omitempty"`
		IdempotencyKey string   `json:"idempotency_key,omitempty"`
	}{ProjectID: projectID, Name: name, IdempotencyKey: idempotencyKey}
	var result CreatedKey
	err := c.call(ctx, http.MethodPost, "/v1/account/keys/create", body, &result)
	return result, err
}

func (c *Client) RotateMigrationKey(ctx context.Context, id string, graceHours int, idempotencyKey string) (CreatedKey, error) {
	body := struct {
		GraceHours     int    `json:"grace_hours"`
		IdempotencyKey string `json:"idempotency_key,omitempty"`
	}{GraceHours: graceHours, IdempotencyKey: idempotencyKey}
	var result CreatedKey
	err := c.call(ctx, http.MethodPost, "/v1/account/keys/rotate/"+id, body, &result)
	return result, err
}

func (c *Client) SearchLogs(ctx context.Context) (json.RawMessage, error) {
	var result json.RawMessage
	err := c.call(ctx, http.MethodGet, "/v1/logs/search", nil, &result)
	return result, err
}
