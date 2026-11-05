package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/platform/logx"
)

type envelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
		Details   struct {
			Fields []apperr.FieldError `json:"fields"`
			Field  string              `json:"field"`
		} `json:"details"`
	} `json:"error"`
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not an error envelope: %v (%s)", err, rec.Body.String())
	}
	return env
}

// withMiddleware wraps h in the production middleware order, logging into buf.
func withMiddleware(h http.Handler, buf *bytes.Buffer) http.Handler {
	logger := logx.New(buf, logx.FormatJSON, slog.LevelDebug)
	return RequestID(logger)(AccessLog(Recoverer(h)))
}

func TestFailWritesEnvelopeWithRequestID(t *testing.T) {
	var logs bytes.Buffer
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Fail(w, r, apperr.ErrNotFound)
	}), &logs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if env.Error.Code != "not_found" || env.Error.RequestID == "" {
		t.Errorf("unexpected envelope: %+v", env.Error)
	}
	if rec.Header().Get(RequestIDHeader) != env.Error.RequestID {
		t.Error("X-Request-Id header must match the envelope requestId")
	}
	if !strings.Contains(logs.String(), `"level":"WARN"`) {
		t.Errorf("4xx should be logged as WARN: %s", logs.String())
	}
}

func TestRequestIDReusesSafeIncomingID(t *testing.T) {
	var logs bytes.Buffer
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { NoContent(w) }), &logs)

	cases := map[string]bool{"web-7f3a9c21": true, "short": false, "bad id with spaces": false, "": false}
	for incoming, reused := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set(RequestIDHeader, incoming)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		got := rec.Header().Get(RequestIDHeader)
		if reused && got != incoming {
			t.Errorf("%q should be reused, got %q", incoming, got)
		}
		if !reused && (got == incoming || len(got) != 36) {
			t.Errorf("%q should be replaced by a UUID, got %q", incoming, got)
		}
	}
}

func TestFailHidesInternalErrors(t *testing.T) {
	var logs bytes.Buffer
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Fail(w, r, errors.New("pq: relation users does not exist"))
	}), &logs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "relation users") {
		t.Error("internal error details leaked to the client")
	}
	if !strings.Contains(logs.String(), "relation users") {
		t.Error("internal error cause must be logged")
	}
}

func TestFailIncludesValidationFields(t *testing.T) {
	rec := httptest.NewRecorder()
	Fail(rec, httptest.NewRequest(http.MethodPost, "/x", nil), apperr.Validation(
		apperr.FieldError{Field: "email", Code: "email", Message: "Ingresa un correo electrónico válido."},
	))

	env := decodeEnvelope(t, rec)
	if rec.Code != http.StatusUnprocessableEntity || len(env.Error.Details.Fields) != 1 || env.Error.Details.Fields[0].Field != "email" {
		t.Errorf("unexpected validation response %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRecovererReturnsEnvelope(t *testing.T) {
	var logs bytes.Buffer
	h := withMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), &logs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError || decodeEnvelope(t, rec).Error.Code != "internal_error" {
		t.Errorf("panic not converted: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "panic recovered") {
		t.Error("panic must be logged")
	}
}

type createUser struct {
	FullName string `json:"fullName" validate:"required,min=3"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"gte=18"`
}

func jsonRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestBind(t *testing.T) {
	cases := []struct {
		name       string
		req        *http.Request
		wantCode   string
		wantStatus int
		wantField  string
	}{
		{"valid", jsonRequest(`{"fullName":"Rosa Guevara","email":"rosa@example.com","age":30}`), "", 0, ""},
		{"wrong content type", httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`)), "unsupported_media_type", 415, ""},
		{"empty body", jsonRequest(``), "bad_request", 400, ""},
		{"malformed", jsonRequest(`{"fullName":`), "bad_request", 400, ""},
		{"unknown field", jsonRequest(`{"fullName":"Rosa","email":"r@x.com","age":20,"role":"admin"}`), "bad_request", 400, ""},
		{"wrong type", jsonRequest(`{"fullName":"Rosa","email":"r@x.com","age":"veinte"}`), "validation_failed", 422, "age"},
		{"two objects", jsonRequest(`{"fullName":"Rosa Guevara","email":"r@x.com","age":20}{}`), "bad_request", 400, ""},
		{"invalid email", jsonRequest(`{"fullName":"Rosa Guevara","email":"nope","age":20}`), "validation_failed", 422, "email"},
		{"too large", jsonRequest(`{"fullName":"` + strings.Repeat("a", MaxBodyBytes) + `"}`), "payload_too_large", 413, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dst createUser
			err := Bind(httptest.NewRecorder(), tc.req, &dst)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			status, body := toBody(err)
			if body.Code != tc.wantCode || status != tc.wantStatus {
				t.Fatalf("got %d %s, want %d %s (%v)", status, body.Code, tc.wantStatus, tc.wantCode, err)
			}
			if tc.wantField != "" {
				appErr, _ := apperr.As(err)
				if len(appErr.Fields) == 0 || appErr.Fields[0].Field != tc.wantField {
					t.Errorf("fields = %+v, want %s", appErr.Fields, tc.wantField)
				}
			}
		})
	}
}

func TestValidateReportsSpanishMessagesByJSONName(t *testing.T) {
	err := Validate(createUser{FullName: "Al", Email: "", Age: 15})
	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %v", err)
	}

	got := map[string]string{}
	for _, f := range appErr.Fields {
		got[f.Field] = f.Message
	}
	want := map[string]string{
		"fullName": "Debe tener al menos 3 caracteres.",
		"email":    "Este campo es obligatorio.",
		"age":      "Debe ser mayor o igual a 18.",
	}
	for field, message := range want {
		if got[field] != message {
			t.Errorf("%s: got %q, want %q", field, got[field], message)
		}
	}
}

func TestParsePage(t *testing.T) {
	page, err := ParsePage(httptest.NewRequest(http.MethodGet, "/x?page=3&pageSize=10", nil))
	if err != nil || page.Number != 3 || page.Size != 10 || page.Offset() != 20 {
		t.Errorf("got %+v, %v", page, err)
	}

	page, err = ParsePage(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil || page.Number != 1 || page.Size != DefaultPageSize {
		t.Errorf("defaults: got %+v, %v", page, err)
	}

	_, err = ParsePage(httptest.NewRequest(http.MethodGet, "/x?page=0&pageSize=500", nil))
	appErr, ok := apperr.As(err)
	if !ok || len(appErr.Fields) != 2 {
		t.Errorf("expected two field errors, got %v", err)
	}
}

func TestNewListNeverReturnsNullData(t *testing.T) {
	body, _ := json.Marshal(NewList[string](nil, Page{Number: 1, Size: 25}, 0))
	if !strings.Contains(string(body), `"data":[]`) {
		t.Errorf("empty list must serialize as []: %s", body)
	}
}

func TestLevelFor(t *testing.T) {
	cases := map[int]slog.Level{200: slog.LevelInfo, 304: slog.LevelInfo, 404: slog.LevelWarn, 422: slog.LevelWarn, 503: slog.LevelError}
	for status, want := range cases {
		if got := levelFor(status); got != want {
			t.Errorf("levelFor(%d) = %v, want %v", status, got, want)
		}
	}
}
