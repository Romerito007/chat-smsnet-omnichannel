// Package mcp is the Model Context Protocol client over the Streamable HTTP
// transport (JSON-RPC 2.0 over HTTP POST, JSON or SSE responses). It discovers a
// server's tools and invokes them generically — it never knows any tool name, so
// adding a new MCP server is pure configuration.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	mcpcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
)

// protocolVersion is the MCP revision advertised on initialize.
const protocolVersion = "2024-11-05"

// defaultTimeout bounds every MCP HTTP call so a slow server never blocks chat.
const defaultTimeout = 20 * time.Second

// Client is the Streamable HTTP MCP client.
type Client struct {
	http *http.Client
	ids  atomic.Int64
}

// NewClient builds the client.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: defaultTimeout}}
}

// ListTools performs the initialize handshake then tools/list.
func (c *Client) ListTools(ctx context.Context, conn *entity.ServerConnection) ([]mcpcontracts.ToolSpec, error) {
	session, err := c.initialize(ctx, conn)
	if err != nil {
		return nil, err
	}
	raw, err := c.rpc(ctx, conn, session, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list: %w", err)
	}
	specs := make([]mcpcontracts.ToolSpec, 0, len(out.Tools))
	for _, t := range out.Tools {
		specs = append(specs, mcpcontracts.ToolSpec{Name: t.Name, Description: t.Description, Schema: t.InputSchema})
	}
	return specs, nil
}

// CallTool performs the handshake then tools/call.
func (c *Client) CallTool(ctx context.Context, conn *entity.ServerConnection, tool string, args map[string]any) (mcpcontracts.CallResult, error) {
	session, err := c.initialize(ctx, conn)
	if err != nil {
		return mcpcontracts.CallResult{}, err
	}
	if args == nil {
		args = map[string]any{}
	}
	raw, err := c.rpc(ctx, conn, session, "tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return mcpcontracts.CallResult{}, err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return mcpcontracts.CallResult{}, fmt.Errorf("mcp: decode tools/call: %w", err)
	}
	var b strings.Builder
	for _, blk := range out.Content {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return mcpcontracts.CallResult{Text: b.String(), IsError: out.IsError}, nil
}

// initialize negotiates the session and returns the Mcp-Session-Id (empty for
// stateless servers).
func (c *Client) initialize(ctx context.Context, conn *entity.ServerConnection) (string, error) {
	params := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "chat-smsnet-omnichannel", "version": "1.0"},
	}
	_, session, err := c.do(ctx, conn, "", "initialize", params)
	if err != nil {
		return "", err
	}
	return session, nil
}

// rpc issues a JSON-RPC call carrying the session id and returns the result.
func (c *Client) rpc(ctx context.Context, conn *entity.ServerConnection, session, method string, params any) (json.RawMessage, error) {
	result, _, err := c.do(ctx, conn, session, method, params)
	return result, err
}

type jsonRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, conn *entity.ServerConnection, session, method string, params any) (json.RawMessage, string, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": c.ids.Add(1), "method": method, "params": params,
	})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conn.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if conn.AuthHeader != "" && conn.AuthToken != "" {
		req.Header.Set(conn.AuthHeader, conn.AuthToken)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("mcp: call %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("mcp: %s returned %d", method, resp.StatusCode)
	}
	newSession := resp.Header.Get("Mcp-Session-Id")
	if newSession == "" {
		newSession = session
	}

	payload := extractJSON(resp.Header.Get("Content-Type"), raw)
	if len(payload) == 0 {
		// A notification/empty 202 (no body) is a valid success with no result.
		return nil, newSession, nil
	}
	var rpc jsonRPCResponse
	if err := json.Unmarshal(payload, &rpc); err != nil {
		return nil, "", fmt.Errorf("mcp: decode response: %w", err)
	}
	if rpc.Error != nil {
		return nil, "", fmt.Errorf("mcp: %s error %d: %s", method, rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, newSession, nil
}

// extractJSON returns the JSON-RPC payload from a JSON body, or — for an SSE
// response — the data of the last "data:" event.
func extractJSON(contentType string, body []byte) []byte {
	if strings.Contains(contentType, "text/event-stream") {
		var last []byte
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimRight(line, "\r")
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				last = []byte(strings.TrimSpace(data))
			}
		}
		return last
	}
	return bytes.TrimSpace(body)
}

var _ mcpcontracts.Client = (*Client)(nil)
