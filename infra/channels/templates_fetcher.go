package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/sign"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// TemplateFetcher fetches a channel's WhatsApp templates from its gateway over HTTP,
// signing the GET with the channel's outbound secret using the SAME HMAC scheme as
// outbound delivery (so the gateway's existing verifier accepts it).
//
// HMAC of the empty-body GET (what the gateway must validate):
//
//	timestamp = current epoch millis, as a string
//	Signature = hex( HMAC_SHA256( outbound_secret, timestamp + "." + "" ) )
//
// i.e. the signed material is "<timestamp>." (the timestamp, a literal dot, then an
// EMPTY body). Sent as headers Timestamp and Signature — identical to the outbound
// envelope's signing, just over an empty body.
type TemplateFetcher struct {
	client *http.Client
}

// NewTemplateFetcher builds the fetcher.
func NewTemplateFetcher() *TemplateFetcher {
	return &TemplateFetcher{client: infrahttp.New(10 * time.Second)}
}

// FetchTemplates GETs {origin(outbound_url)}/channels/{type}/templates and returns
// the parsed mirror. A non-2xx or transport error is returned as an error so the
// caller keeps the existing templates (only a successful fetch replaces them).
func (f *TemplateFetcher) FetchTemplates(ctx context.Context, conn *chentity.ChannelConnection) ([]chentity.WhatsAppTemplate, error) {
	endpoint, err := templatesURL(conn)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build templates request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	// Sign the empty-body GET exactly like an outbound delivery (Timestamp +
	// Signature = HMAC(secret, "<ts>." )). sign.Payload writes ts + "." + body, and
	// body is nil here, so it is byte-for-byte the empty-body signature.
	if conn.Secret != "" {
		ts := strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
		req.Header.Set("Timestamp", ts)
		req.Header.Set("Signature", sign.Payload(conn.Secret, ts, nil))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch templates from gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gateway returned %d fetching templates: %s", resp.StatusCode, truncateBody(body, 256))
	}

	var payload struct {
		Templates []wireTemplate `json:"templates"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode templates response: %w", err)
	}
	return wireTemplatesToEntity(payload.Templates), nil
}

// templatesURL derives the gateway templates endpoint from the channel's outbound
// URL origin: {scheme}://{host}/channels/{type}/templates.
func templatesURL(conn *chentity.ChannelConnection) (string, error) {
	base := strings.TrimSpace(conn.BaseURL)
	if base == "" {
		return "", fmt.Errorf("channel has no outbound_url to fetch templates from")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid channel outbound_url %q", base)
	}
	return u.Scheme + "://" + u.Host + "/channels/" + url.PathEscape(string(conn.Type)) + "/templates", nil
}

func truncateBody(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// ── gateway wire shape (mirrors the presenter WhatsAppTemplate DTO) ─────────────

type wireTemplate struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Language string       `json:"language"`
	Category string       `json:"category"`
	Body     wireBody     `json:"body"`
	Header   *wireHeader  `json:"header"`
	Buttons  []wireButton `json:"buttons"`
	Footer   string       `json:"footer"`
}

type wireBody struct {
	Text      string         `json:"text"`
	Variables []wireVariable `json:"variables"`
}

type wireVariable struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Example string `json:"example"`
}

type wireHeader struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type wireButton struct {
	Type string `json:"type"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

func wireTemplatesToEntity(in []wireTemplate) []chentity.WhatsAppTemplate {
	out := make([]chentity.WhatsAppTemplate, 0, len(in))
	for _, t := range in {
		vars := make([]chentity.WhatsAppTemplateVariable, len(t.Body.Variables))
		for i, v := range t.Body.Variables {
			vars[i] = chentity.WhatsAppTemplateVariable{Key: v.Key, Label: v.Label, Example: v.Example}
		}
		btns := make([]chentity.WhatsAppTemplateButton, len(t.Buttons))
		for i, b := range t.Buttons {
			btns[i] = chentity.WhatsAppTemplateButton{Type: b.Type, Text: b.Text, URL: b.URL}
		}
		e := chentity.WhatsAppTemplate{
			ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category,
			Body:    chentity.WhatsAppTemplateBody{Text: t.Body.Text, Variables: vars},
			Buttons: btns, Footer: t.Footer,
		}
		if t.Header != nil {
			e.Header = &chentity.WhatsAppTemplateHeader{Type: t.Header.Type, Text: t.Header.Text}
		}
		out = append(out, e)
	}
	return out
}

var _ chcontracts.TemplateFetcher = (*TemplateFetcher)(nil)
