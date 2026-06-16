package reports_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	rcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/reports"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// fakeReportSvc implements rcontracts.ReportService; only Export is exercised. It
// writes a real file to the shared store and returns a signed URL, so the test
// covers the controller + the real LocalFileStore sign/resolve/open round-trip.
type fakeReportSvc struct {
	files rcontracts.FileStore
	body  []byte
}

func (s *fakeReportSvc) Export(_ context.Context, report, format string, _ rcontracts.Filter) (rcontracts.ExportResult, error) {
	key := "reports/t1/" + report + "." + format
	if err := s.files.Save(key, s.body); err != nil {
		return rcontracts.ExportResult{}, err
	}
	url, exp, err := s.files.SignedURL(key, time.Hour)
	if err != nil {
		return rcontracts.ExportResult{}, err
	}
	return rcontracts.ExportResult{Report: report, Format: format, Filename: report + "." + format, DownloadURL: url, ExpiresAt: exp, Bytes: len(s.body)}, nil
}

func (s *fakeReportSvc) Overview(context.Context, rcontracts.Filter) (rcontracts.Overview, error) {
	return rcontracts.Overview{}, nil
}
func (s *fakeReportSvc) Conversations(context.Context, rcontracts.Filter) (rcontracts.ConversationsReport, error) {
	return rcontracts.ConversationsReport{}, nil
}
func (s *fakeReportSvc) Agents(context.Context, rcontracts.Filter) (rcontracts.AgentsReport, error) {
	return rcontracts.AgentsReport{}, nil
}
func (s *fakeReportSvc) Sectors(context.Context, rcontracts.Filter) (rcontracts.SectorsReport, error) {
	return rcontracts.SectorsReport{}, nil
}
func (s *fakeReportSvc) Copilot(context.Context, rcontracts.Filter) (rcontracts.CopilotReport, error) {
	return rcontracts.CopilotReport{}, nil
}
func (s *fakeReportSvc) Automation(context.Context, rcontracts.Filter) (rcontracts.AutomationReport, error) {
	return rcontracts.AutomationReport{}, nil
}
func (s *fakeReportSvc) SLA(context.Context, rcontracts.Filter) (rcontracts.SLAReport, error) {
	return rcontracts.SLAReport{}, nil
}
func (s *fakeReportSvc) CSAT(context.Context, rcontracts.Filter) (rcontracts.CSATReport, error) {
	return rcontracts.CSATReport{}, nil
}

var _ rcontracts.ReportService = (*fakeReportSvc)(nil)

// statRowsSvc is a ReportService returning fixed agent/sector rows with raw ids,
// to exercise the controller's display-name enrichment.
type statRowsSvc struct{ *fakeReportSvc }

func (statRowsSvc) Agents(context.Context, rcontracts.Filter) (rcontracts.AgentsReport, error) {
	return rcontracts.AgentsReport{Agents: []rcontracts.AgentStat{
		{AgentID: "u1", Conversations: 3},
		{AgentID: "u2", Conversations: 1},
	}}, nil
}
func (statRowsSvc) Sectors(context.Context, rcontracts.Filter) (rcontracts.SectorsReport, error) {
	return rcontracts.SectorsReport{Sectors: []rcontracts.SectorStat{
		{SectorID: "s1", Conversations: 4},
		{SectorID: "", Conversations: 2}, // sector-less: stays nameless
	}}, nil
}

type fakeAgentDir struct{}

func (fakeAgentDir) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if id == "u1" {
			out[id] = shared.DisplayCard{Name: "Ana", AvatarURL: "https://cdn/ana.png"}
		}
	}
	return out, nil // u2 intentionally unresolved
}

type fakeSectorDir struct{}

func (fakeSectorDir) Names(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if id == "s1" {
			out[id] = "Suporte"
		}
	}
	return out, nil
}

// TestAgentsReport_EnrichesNames proves /reports/agents fills the resolved name +
// avatar (and leaves unresolved rows with just the id).
func TestAgentsReport_EnrichesNames(t *testing.T) {
	tm := httpharness.Tokens()
	files := storage.NewLocalFileStoreAt(t.TempDir(), "test-secret", "http://api.test", "/v1/reports/downloads/")
	ctl := reports.NewController(statRowsSvc{&fakeReportSvc{}}, files).SetDirectories(fakeAgentDir{}, fakeSectorDir{})

	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.ReportView)).Get("/reports/agents", ctl.Agents)
		p.With(middleware.RequirePermission(authz.ReportView)).Get("/reports/sectors", ctl.Sectors)
	})

	tok := httpharness.Token(t, tm, "t1", "u1", authz.ReportView)

	rec := httpharness.Do(t, r, http.MethodGet, "/reports/agents", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("agents status = %d (%s)", rec.Code, rec.Body.String())
	}
	var ag struct {
		Agents []struct {
			AgentID   string `json:"agent_id"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"agents"`
	}
	httpharness.DecodeJSON(t, rec, &ag)
	if len(ag.Agents) != 2 || ag.Agents[0].Name != "Ana" || ag.Agents[0].AvatarURL == "" {
		t.Fatalf("agent not enriched: %+v", ag.Agents)
	}
	if ag.Agents[1].Name != "" {
		t.Errorf("unresolved agent should stay nameless: %+v", ag.Agents[1])
	}

	rec = httpharness.Do(t, r, http.MethodGet, "/reports/sectors", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sectors status = %d (%s)", rec.Code, rec.Body.String())
	}
	var se struct {
		Sectors []struct {
			SectorID string `json:"sector_id"`
			Name     string `json:"name"`
		} `json:"sectors"`
	}
	httpharness.DecodeJSON(t, rec, &se)
	if len(se.Sectors) != 2 || se.Sectors[0].Name != "Suporte" {
		t.Fatalf("sector not enriched: %+v", se.Sectors)
	}
	if se.Sectors[1].Name != "" {
		t.Errorf("sector-less row should stay nameless: %+v", se.Sectors[1])
	}
}

// TestExportThenDownload_RoundTrip proves a report export produces a real file
// reachable via the signed download URL.
func TestExportThenDownload_RoundTrip(t *testing.T) {
	tm := httpharness.Tokens()
	files := storage.NewLocalFileStoreAt(t.TempDir(), "test-secret", "http://api.test", "/v1/reports/downloads/")
	body := []byte("metric,value\ntotal_conversations,7\n")
	ctl := reports.NewController(&fakeReportSvc{files: files, body: body}, files)

	r := chi.NewRouter()
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.With(middleware.RequirePermission(authz.ReportExport)).Post("/reports/export", ctl.Export)
	})
	r.Get("/v1/reports/downloads/{token}", ctl.Download)

	tok := httpharness.Token(t, tm, "t1", "u1", authz.ReportExport)
	rec := httpharness.Do(t, r, http.MethodPost, "/reports/export?report=overview&format=csv", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d (%s)", rec.Code, rec.Body.String())
	}
	var res struct {
		DownloadURL string `json:"download_url"`
		Format      string `json:"format"`
	}
	httpharness.DecodeJSON(t, rec, &res)
	if res.Format != "csv" || res.DownloadURL == "" {
		t.Fatalf("unexpected export response: %+v", res)
	}

	// Follow the signed URL: the public download must return the exact file bytes.
	idx := strings.Index(res.DownloadURL, "/v1/reports/downloads/")
	if idx < 0 {
		t.Fatalf("download url not pointed at reports endpoint: %s", res.DownloadURL)
	}
	dl := httpharness.Do(t, r, http.MethodGet, res.DownloadURL[idx:], "", nil)
	if dl.Code != http.StatusOK {
		t.Fatalf("download status = %d (%s)", dl.Code, dl.Body.String())
	}
	if dl.Body.String() != string(body) {
		t.Errorf("downloaded bytes mismatch:\n got %q\nwant %q", dl.Body.String(), string(body))
	}
	if ct := dl.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Errorf("content-type = %q, want text/csv", ct)
	}
}
