package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ─── OK / Created ───────────────────────────────────────────────────────────

func TestOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OKResp(c, map[string]string{"foo": "bar"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["success"] != true {
		t.Error("expected success:true")
	}
}

func TestCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	CreatedResp(c, gin.H{"id": 1})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

// ─── 4xx responses ─────────────────────────────────────────────────────────

func TestBadRequest_GenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	BadRequestResp(c, errors.New("some raw error"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "some raw error") {
		t.Error("raw error message should NOT leak in generic BadRequest")
	}
}

func TestBadRequest_AppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	BadRequestResp(c, NewBadRequestf("invalid input: %s", "missing field"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "missing field") {
		t.Error("AppError formatted message should appear")
	}
}

func TestUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	UnauthorizedResp(c, "Please login")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Please login") {
		t.Error("expected custom message in body")
	}
}

func TestUnauthorized_EmptyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	UnauthorizedResp(c, "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Authentication required") {
		t.Error("expected default message when empty")
	}
}

func TestForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ForbiddenResp(c, "")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestNotFound_(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NotFoundResp(c, "User")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "User not found") {
		t.Error("expected resource name in body")
	}
}

func TestTooManyRequests_WithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	TooManyRequestsResp(c, 30)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "30" {
		t.Errorf("expected Retry-After: 30, got %q", got)
	}
}

// ─── 5xx: CRITICAL — must NOT leak err.Error() ─────────────────────────────

func TestInternalError_NoLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Use a real Gin engine so FullPath() works
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		secretErr := errors.New("connection to postgres://admin:hunter2@db.local:5432 failed: FATAL: permission denied")
		InternalErrorResp(c, secretErr, "ChatUseCase.SaveMessage")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	body := w.Body.String()
	// CRITICAL: must NOT leak ANY part of the error message
	if strings.Contains(body, "postgres://admin") {
		t.Error("❌ SECURITY LEAK: error message exposed DB credentials in client response")
	}
	if strings.Contains(body, "hunter2") {
		t.Error("❌ SECURITY LEAK: error message exposed password in client response")
	}
	if strings.Contains(body, "FATAL") {
		t.Error("❌ SECURITY LEAK: error message exposed SQL error in client response")
	}
	if !strings.Contains(body, "internal error") {
		t.Error("expected generic internal error message")
	}
}

func TestInternalError_LogsInternally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		err := errors.New("database connection refused")
		InternalErrorResp(c, err, "SomeOp")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "database connection refused") {
		t.Error("internal error message should NOT leak in body")
	}
}

// ─── ValidationError ────────────────────────────────────────────────────────

func TestValidationError_BindingErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Plain error → falls through to BadRequestMessageResp
	ValidationErrorResp(c, errors.New("invalid json"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid json") {
		t.Error("expected error message in body")
	}
}

// ─── AppError ───────────────────────────────────────────────────────────────

func TestAppError_WithDetails(t *testing.T) {
	err := NewBadRequestf("invalid input")
	err = err.WithDetails(map[string]string{"field": "email"})

	if err.Details == nil {
		t.Error("expected details to be set")
	}
}

func TestAppError_Unwrap(t *testing.T) {
	original := errors.New("internal db error")
	appErr := NewInternalError(original)

	if errors.Unwrap(appErr) != original {
		t.Error("expected Unwrap to return original error")
	}
}

func TestAppError_ErrorString(t *testing.T) {
	appErr := NewBadRequestf("invalid input: %s", "missing email")
	if !strings.Contains(appErr.Error(), "invalid input") {
		t.Error("expected formatted message")
	}
	if !strings.Contains(appErr.Error(), "missing email") {
		t.Error("expected substituted arg")
	}
}