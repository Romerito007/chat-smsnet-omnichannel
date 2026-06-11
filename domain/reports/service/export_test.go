package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
)

// memFileStore is an in-memory contracts.FileStore for tests: it records the
// saved bytes and hands back a deterministic signed URL + a working round-trip.
type memFileStore struct {
	saved map[string][]byte
}

func newMemFileStore() *memFileStore { return &memFileStore{saved: map[string][]byte{}} }

func (m *memFileStore) Save(key string, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.saved[key] = cp
	return nil
}
func (m *memFileStore) SignedURL(key string, ttl time.Duration) (string, time.Time, error) {
	return "https://api.test/v1/reports/downloads/tok-" + key, time.Now().Add(ttl), nil
}
func (m *memFileStore) Resolve(token string) (string, error) {
	return strings.TrimPrefix(token, "tok-"), nil
}
func (m *memFileStore) Open(key string) ([]byte, string, error) {
	data, ok := m.saved[key]
	if !ok {
		return nil, "", apperror.NotFound("nf")
	}
	return data, "report.json", nil
}

func exportService(t *testing.T) (*Service, *memFileStore) {
	repo := &fakeRepo{total: 7, msgs: 30, byStatus: []contracts.Bucket{{Key: "open", Count: 4}}}
	svc := NewService(repo, clk(t))
	files := newMemFileStore()
	svc.SetFileStore(files, time.Hour)
	return svc, files
}

func TestExport_JSON_RealFileAndSignedURL(t *testing.T) {
	svc, files := exportService(t)
	res, err := svc.Export(ctxT(), "overview", "json", contracts.Filter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if res.Format != "json" || !strings.HasSuffix(res.Filename, ".json") {
		t.Errorf("unexpected result meta: %+v", res)
	}
	if res.DownloadURL == "" || res.Bytes == 0 {
		t.Errorf("expected a signed url and non-empty file: %+v", res)
	}
	if len(files.saved) != 1 {
		t.Fatalf("expected exactly one stored file, got %d", len(files.saved))
	}
	// The stored bytes are valid JSON containing the computed metric.
	var got map[string]any
	for _, data := range files.saved {
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("stored file is not valid JSON: %v", err)
		}
	}
	if got["total_conversations"] != float64(7) {
		t.Errorf("report content missing/incorrect: %v", got["total_conversations"])
	}
}

func TestExport_CSV_HasHeaderAndRows(t *testing.T) {
	svc, files := exportService(t)
	res, err := svc.Export(ctxT(), "overview", "csv", contracts.Filter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if res.Format != "csv" || !strings.HasSuffix(res.Filename, ".csv") {
		t.Errorf("unexpected csv meta: %+v", res)
	}
	var content string
	for _, data := range files.saved {
		content = string(data)
	}
	if !strings.HasPrefix(content, "metric,value\n") {
		t.Errorf("csv missing header row:\n%s", content)
	}
	if !strings.Contains(content, "total_conversations,7") {
		t.Errorf("csv missing flattened metric:\n%s", content)
	}
}

func TestExport_UnknownReport(t *testing.T) {
	svc, _ := exportService(t)
	if _, err := svc.Export(ctxT(), "bogus", "json", contracts.Filter{}); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExport_NoFileStoreConfigured(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, clk(t)) // no SetFileStore
	if _, err := svc.Export(ctxT(), "overview", "json", contracts.Filter{}); apperror.From(err).Code != apperror.CodeInternal {
		t.Fatalf("expected internal error without a file store, got %v", err)
	}
}
