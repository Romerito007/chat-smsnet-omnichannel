package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestFromPreservesAppError(t *testing.T) {
	orig := NotFound("missing")
	got := From(orig)
	if got != orig {
		t.Fatal("From should return the same AppError instance")
	}
}

func TestFromWrapsUnknown(t *testing.T) {
	cause := errors.New("boom")
	got := From(cause)
	if got.Code != CodeInternal {
		t.Errorf("expected internal_error, got %s", got.Code)
	}
	if !errors.Is(got, cause) {
		t.Error("expected wrapped cause to be retrievable via errors.Is")
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	cases := map[Code]int{
		CodeValidation:   http.StatusBadRequest,
		CodeUnauthorized: http.StatusUnauthorized,
		CodeForbidden:    http.StatusForbidden,
		CodeNotFound:     http.StatusNotFound,
		CodeConflict:     http.StatusConflict,
		CodeRateLimited:  http.StatusTooManyRequests,
		CodeInternal:     http.StatusInternalServerError,
	}
	for code, status := range cases {
		if got := New(code, "x").HTTPStatus(); got != status {
			t.Errorf("%s => %d, want %d", code, got, status)
		}
	}
}

func TestWithDetailsIsImmutable(t *testing.T) {
	base := Validation("bad")
	withA := base.WithDetails(map[string]any{"field": "email"})
	if base.Details != nil {
		t.Error("base error should not be mutated")
	}
	if withA.Details["field"] != "email" {
		t.Error("details not attached to copy")
	}
}
