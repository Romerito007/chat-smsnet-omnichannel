package automation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// FlowClient calls the external flow system over HTTP, signing the request body
// with the integration secret (X-Signature: HMAC-SHA256).
type FlowClient struct {
	client *http.Client
}

// NewFlowClient builds the client.
func NewFlowClient() *FlowClient {
	return &FlowClient{client: infrahttp.New(15 * time.Second)}
}

// flowResponse is the external flow's start response.
type flowResponse struct {
	ExternalRunID string              `json:"external_run_id"`
	Decision      *contracts.Decision `json:"decision"`
}

// Start posts the input to the flow's base URL and parses the response. A
// per-request timeout is taken from the integration.
func (c *FlowClient) Start(ctx context.Context, integration *entity.AutomationIntegration, input contracts.FlowInput) (contracts.FlowStartResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return contracts.FlowStartResult{}, err
	}

	timeout := time.Duration(integration.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, integration.BaseURL, bytes.NewReader(body))
	if err != nil {
		return contracts.FlowStartResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "sha256="+sign(body, integration.Secret))

	resp, err := c.client.Do(req)
	if err != nil {
		return contracts.FlowStartResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return contracts.FlowStartResult{}, fmt.Errorf("flow returned status %d", resp.StatusCode)
	}

	var fr flowResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return contracts.FlowStartResult{}, err
	}
	return contracts.FlowStartResult{ExternalRunID: fr.ExternalRunID, Decision: fr.Decision}, nil
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

var _ contracts.FlowClient = (*FlowClient)(nil)
