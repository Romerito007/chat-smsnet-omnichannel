package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestConversationSubrouters_NoMountConflict reproduces the route nesting used in
// production: the /conversations subrouter coexisting with deeper per-conversation
// subrouters (external, mcp, copilot approvals). It must register without a chi
// panic and route each path to the right handler.
func TestConversationSubrouters_NoMountConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("router registration panicked (mount conflict): %v", r)
		}
	}()

	hit := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(name)) }
	}
	r := chi.NewRouter()
	// conversations subrouter (registerConversationRoutes shape).
	r.Group(func(p chi.Router) {
		p.Route("/conversations", func(cv chi.Router) {
			cv.Get("/{id}", hit("get"))
			cv.Post("/{id}/messages", hit("messages"))
		})
	})
	// external subrouter (registerExternalRoutes shape).
	r.Group(func(p chi.Router) {
		p.Route("/conversations/{id}/external", func(ex chi.Router) {
			ex.Get("/cliente", hit("cliente"))
		})
	})
	// mcp + approvals subrouters (registerMCPRoutes shape).
	r.Group(func(p chi.Router) {
		p.Route("/conversations/{id}/mcp", func(m chi.Router) {
			m.Get("/tools", hit("tools"))
			m.Post("/run", hit("run"))
		})
		p.Route("/conversations/{id}/copilot/approvals", func(a chi.Router) {
			a.Post("/{approvalID}", hit("decide"))
		})
	})

	cases := []struct{ method, path, want string }{
		{http.MethodGet, "/conversations/c1", "get"},
		{http.MethodPost, "/conversations/c1/messages", "messages"},
		{http.MethodGet, "/conversations/c1/external/cliente", "cliente"},
		{http.MethodGet, "/conversations/c1/mcp/tools", "tools"},
		{http.MethodPost, "/conversations/c1/mcp/run", "run"},
		{http.MethodPost, "/conversations/c1/copilot/approvals/a1", "decide"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.want {
			t.Errorf("%s %s → code=%d body=%q, want %q", tc.method, tc.path, rec.Code, rec.Body.String(), tc.want)
		}
	}
}
