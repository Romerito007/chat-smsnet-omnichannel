package start_routines

import (
	"context"
	"net/http"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
)

// logIntegrationsHealth best-effort probes the configured SMSNET integration
// endpoints (HTTP gateway + the two MCP servers) and logs reachability at INFO.
// It is non-fatal and runs in the background so a slow/unreachable host never
// delays startup. Only the env-var name is logged (never the URL/key). These
// hosts are expected on a private network and must never be exposed publicly.
func logIntegrationsHealth(ctx context.Context, c *container.Container) {
	targets := []struct{ name, url string }{
		{"ISP_GATEWAY_API_HOST", c.Config.ProviderHub.GatewayAPIHost},
		{"SMSNET_MCP_CONSULTAS_URL", c.Config.MCP.ConsultasURL},
		{"SMSNET_MCP_OPERACOES_URL", c.Config.MCP.OperacoesURL},
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for _, t := range targets {
		if t.url == "" {
			continue
		}
		reachable := false
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil); err == nil {
			if resp, derr := client.Do(req); derr == nil {
				_ = resp.Body.Close()
				reachable = true // any HTTP response means the host is up
			}
		}
		c.Logger.Info("smsnet integration endpoint",
			"target", t.name, "configured", true, "reachable", reachable,
			"note", "private network only — never expose to the internet")
	}
}
