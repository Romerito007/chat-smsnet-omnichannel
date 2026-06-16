package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
)

// render computes the named report and serializes it to bytes in the requested
// format, returning the file bytes and the file extension. JSON emits the full
// typed report; CSV flattens it to key,value rows (a stable, universal table).
func (s *Service) render(ctx context.Context, report, format string, f contracts.Filter) ([]byte, string, error) {
	payload, err := s.reportPayload(ctx, report, f)
	if err != nil {
		return nil, "", err
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, "", apperror.Internal("could not encode report json").Wrap(err)
		}
		return data, "json", nil
	case "csv":
		data, err := toCSV(payload)
		if err != nil {
			return nil, "", apperror.Internal("could not encode report csv").Wrap(err)
		}
		return data, "csv", nil
	default:
		return nil, "", apperror.Validation("unsupported format").
			WithDetails(map[string]any{"format": "must be json or csv"})
	}
}

// reportPayload runs the aggregation for the named report.
func (s *Service) reportPayload(ctx context.Context, report string, f contracts.Filter) (any, error) {
	switch strings.ToLower(strings.TrimSpace(report)) {
	case "", "overview":
		return s.Overview(ctx, f)
	case "conversations":
		return s.Conversations(ctx, f)
	case "agents":
		return s.Agents(ctx, f)
	case "sectors":
		return s.Sectors(ctx, f)
	case "copilot":
		return s.Copilot(ctx, f)
	case "automation":
		return s.Automation(ctx, f)
	case "sla":
		return s.SLA(ctx, f)
	case "csat":
		return s.CSAT(ctx, f)
	default:
		return nil, apperror.Validation("unknown report").
			WithDetails(map[string]any{"report": "overview|conversations|agents|sectors|copilot|automation|sla|csat"})
	}
}

// toCSV flattens a report (via its JSON shape) into deterministic key,value rows.
// Nested objects/arrays use dot/index paths, so every report type serializes to a
// real, spreadsheet-openable table regardless of its shape.
func toCSV(payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	flat := map[string]string{}
	flatten("", tree, flat)

	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"metric", "value"})
	for _, k := range keys {
		_ = w.Write([]string{k, flat[k]})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func flatten(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			flatten(join(prefix, k), child, out)
		}
	case []any:
		for i, child := range t {
			flatten(join(prefix, strconv.Itoa(i)), child, out)
		}
	case float64:
		// json numbers decode as float64; render integers without a decimal point.
		if t == float64(int64(t)) {
			out[prefix] = strconv.FormatInt(int64(t), 10)
		} else {
			out[prefix] = strconv.FormatFloat(t, 'f', -1, 64)
		}
	case nil:
		out[prefix] = ""
	default:
		out[prefix] = fmt.Sprintf("%v", t)
	}
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
