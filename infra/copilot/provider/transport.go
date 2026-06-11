package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// defaultTimeout bounds every provider HTTP call so a slow/unreachable provider
// never blocks the agent. The caller's context can shorten it further.
const defaultTimeout = 30 * time.Second

// sharedClient is reused across calls (connection pooling) with a hard timeout.
var sharedClient = &http.Client{Timeout: defaultTimeout}

// notConfigured is the friendly error when a tenant selected a provider but set
// no API key. The service maps any provider error to a user-safe message.
func notConfigured(p entity.Provider) error {
	return fmt.Errorf("provider %s is not configured (no API key)", p)
}

// postJSON issues a JSON POST and returns the raw response body. A non-2xx status
// is returned as an error carrying a truncated body for the AILog.
func postJSON(ctx context.Context, url string, headers map[string]string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call provider: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// orDefault returns v when non-empty, else def.
func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// maxTokensOr returns n when positive, else def.
func maxTokensOr(n, def int) int {
	if n > 0 {
		return n
	}
	return def
}

// classifyCategories normalizes the model's classify output to the configured
// category list (case-insensitive), falling back to the trimmed text.
func classifyCategories(text, instruction string) []string {
	choice := strings.TrimSpace(text)
	for _, c := range parseCategories(instruction) {
		if strings.EqualFold(strings.TrimSpace(c), choice) || strings.Contains(strings.ToLower(choice), strings.ToLower(strings.TrimSpace(c))) {
			return []string{strings.TrimSpace(c)}
		}
	}
	if choice == "" {
		return nil
	}
	return []string{choice}
}
