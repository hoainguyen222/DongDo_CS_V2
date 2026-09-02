package httpx

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func init() {
	// Initialize a discard logger so tests don't panic when slog is called
	if slog.Default() == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	}
}

// MockLogBuffer captures log output for test assertions.
func MockLogBuffer() (*bytes.Buffer, func()) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelError}))
	old := slog.Default()
	slog.SetDefault(logger)
	return buf, func() { slog.SetDefault(old) }
}

// ─── Success responses ─────────────────────────────────────────────────────

// OKResp sends 200 with data wrapped in {success, data}.
func OKResp(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// CreatedResp sends 201 with data.
func CreatedResp(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    data,
	})
}

// ─── 4xx responses ─────────────────────────────────────────────────────────

// BadRequestResp sends 400. If err is *AppError, uses its Code/Message/Details.
// Otherwise returns generic message.
func BadRequestResp(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   appErr,
		})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"error": gin.H{
			"code":    ErrCodeBadRequest,
			"message": "Bad request",
		},
	})
}

// BadRequestMessageResp sends 400 with a custom safe message.
func BadRequestMessageResp(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"error": gin.H{
			"code":    ErrCodeBadRequest,
			"message": message,
		},
	})
}

// UnauthorizedResp sends 401 with custom message.
func UnauthorizedResp(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"error":   NewUnauthorized(message),
	})
}

// ForbiddenResp sends 403 with custom message.
func ForbiddenResp(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"error":   NewForbidden(message),
	})
}

// NotFoundResp sends 404 with resource name.
func NotFoundResp(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"error":   NewNotFound(resource),
	})
}

// ConflictResp sends 409.
func ConflictResp(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, gin.H{
		"success": false,
		"error": gin.H{
			"code":    ErrCodeConflict,
			"message": message,
		},
	})
}

// TooManyRequestsResp sends 429 with Retry-After header.
func TooManyRequestsResp(c *gin.Context, retryAfter int) {
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.JSON(http.StatusTooManyRequests, gin.H{
		"success": false,
		"error":   NewTooManyRequests(retryAfter),
	})
}

// ValidationErrorResp formats gin binding errors nicely for client.
func ValidationErrorResp(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fieldErrors := make(map[string]string)
		for _, fe := range ve {
			fieldErrors[fe.Field()] = fe.Tag()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    ErrCodeValidationFailed,
				"message": "Input validation failed",
				"details": fieldErrors,
			},
		})
		return
	}
	BadRequestMessageResp(c, err.Error())
}

// ─── 5xx responses ─────────────────────────────────────────────────────────

// InternalErrorResp logs the full error server-side and sends a generic message.
// CRITICAL: NEVER expose err.Error() to the client. Full error is logged with request_id.
func InternalErrorResp(c *gin.Context, err error, op string) {
	requestID, _ := c.Get("request_id")
	rid, _ := requestID.(string)

	slog.Error("internal server error",
		slog.String("operation", op),
		slog.String("request_id", rid),
		slog.String("path", c.FullPath()),
		slog.String("method", c.Request.Method),
		slog.Any("error", err),
	)

	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error":   NewInternalError(err), // safe message only
	})
}

// ServiceUnavailableResp sends 503.
func ServiceUnavailableResp(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"success": false,
		"error": gin.H{
			"code":    ErrCodeServiceUnavailable,
			"message": message,
		},
	})
}

// GatewayTimeoutResp sends 504.
func GatewayTimeoutResp(c *gin.Context) {
	c.JSON(http.StatusGatewayTimeout, gin.H{
		"success": false,
		"error": gin.H{
			"code":    ErrCodeGatewayTimeout,
			"message": "Gateway timeout",
		},
	})
}