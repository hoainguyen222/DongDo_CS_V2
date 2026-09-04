package logger

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// ContextKey type for context keys
type ContextKey string

const (
	// ContextKeyRequestID is the context key for request ID
	ContextKeyRequestID ContextKey = "request_id"
	// ContextKeyUserID is the context key for user ID
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeySessionID is the context key for session ID
	ContextKeySessionID ContextKey = "session_id"
	// ContextKeyTenantID is the context key for tenant ID
	ContextKeyTenantID ContextKey = "tenant_id"
	// ContextKeyUserAgent is the context key for user agent
	ContextKeyUserAgent ContextKey = "user_agent"
	// ContextKeyClientIP is the context key for client IP
	ContextKeyClientIP ContextKey = "client_ip"
	// ContextKeyEndpoint is the context key for endpoint
	ContextKeyEndpoint ContextKey = "endpoint"
	// ContextKeyMethod is the context key for HTTP method
	ContextKeyMethod ContextKey = "method"
	// ContextKeyStatusCode is the context key for HTTP status code
	ContextKeyStatusCode ContextKey = "status_code"
	// ContextKeyLatency is the context key for latency
	ContextKeyLatency ContextKey = "latency"
	// ContextKeyRequestBody is the context key for request body
	ContextKeyRequestBody ContextKey = "request_body"
	// ContextKeyResponseBody is the context key for response body
	ContextKeyResponseBody ContextKey = "response_body"
	// ContextKeyError is the context key for error
	ContextKeyError ContextKey = "error"
	// ContextKeyAuditAction is the context key for audit action
	ContextKeyAuditAction ContextKey = "audit_action"
	// ContextKeyAuditResource is the context key for audit resource
	ContextKeyAuditResource ContextKey = "audit_resource"
	// ContextKeyAuditUserID is the context key for audit user ID
	ContextKeyAuditUserID ContextKey = "audit_user_id"
	// ContextKeyAuditIP is the context key for audit IP
	ContextKeyAuditIP ContextKey = "audit_ip"
)

// Config holds logger configuration
type Config struct {
	// LogLevel is the minimum log level (debug, info, warn, error, fatal)
	LogLevel string
	// LogDir is the directory where log files are stored
	LogDir string
	// LogFormat is the log format (json, console)
	LogFormat string
	// EnableConsole enables console output
	EnableConsole bool
	// ConsolePretty enables pretty console output (only for console format)
	ConsolePretty bool
	// EnableFileRotation enables daily file rotation
	EnableFileRotation bool
	// MaxFileSize is the maximum size of each log file in MB (for future use)
	MaxFileSize int
	// MaxBackups is the maximum number of backup files to keep
	MaxBackups int
	// Compress determines if rotated files should be compressed
	Compress bool
	// TimeFormat is the time format for timestamps
	TimeFormat string
}

// Logger wraps zerolog.Logger with additional functionality
type Logger struct {
	zl        zerolog.Logger
	all       *rotatableLogger
	error     *rotatableLogger
	access    *rotatableLogger
	audit     *rotatableLogger
	config    Config
	mu        sync.RWMutex
	ctxLogger *contextLogger
}

// rotatableLogger handles file-based logging with rotation
type rotatableLogger struct {
	mu        sync.Mutex
	filename  string
	file      *os.File
	bufWriter *bufio.Writer
	writer    io.Writer
}

// Write implements io.Writer
func (rl *rotatableLogger) Write(p []byte) (int, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.writer.Write(p)
}

// contextLogger provides context-aware logging
type contextLogger struct {
	logger *Logger
}

// Global logger instance
var (
	defaultLogger *Logger
	once           sync.Once
)

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		LogDir:            getEnv("LOG_DIR", "./logs"),
		LogFormat:         getEnv("LOG_FORMAT", "json"),
		EnableConsole:     getEnv("LOG_ENABLE_CONSOLE", "true") == "true",
		ConsolePretty:     getEnv("LOG_CONSOLE_PRETTY", "false") == "true",
		EnableFileRotation: true,
		MaxFileSize:       100,
		MaxBackups:        30,
		Compress:          true,
		TimeFormat:        time.RFC3339,
	}
}

// New creates a new Logger with the given configuration
func New(cfg Config) (*Logger, error) {
	logger := &Logger{
		config:    cfg,
		ctxLogger: &contextLogger{},
	}

	// Set up zerolog error handling
	zerolog.ErrorFieldName = "error"
	zerolog.ErrorStackFieldName = "stack_trace"
	zerolog.TimeFieldFormat = cfg.TimeFormat
	zerolog.TimestampFieldName = "timestamp"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"

	// Create log directory if it doesn't exist
	if cfg.EnableFileRotation && cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Set up writers
	var writers []io.Writer

	// Console writer
	if cfg.EnableConsole {
		consoleWriter := createConsoleWriter(cfg)
		writers = append(writers, consoleWriter)
	}

	// File writers
	if cfg.EnableFileRotation && cfg.LogDir != "" {
		allLog, err := newRotatableLogger(filepath.Join(cfg.LogDir, "all.log"), cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create all.log writer: %w", err)
		}
		logger.all = allLog
		writers = append(writers, allLog)

		errorLog, err := newRotatableLogger(filepath.Join(cfg.LogDir, "error.log"), cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create error.log writer: %w", err)
		}
		logger.error = errorLog

		accessLog, err := newRotatableLogger(filepath.Join(cfg.LogDir, "access.log"), cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create access.log writer: %w", err)
		}
		logger.access = accessLog

		auditLog, err := newRotatableLogger(filepath.Join(cfg.LogDir, "audit.log"), cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit.log writer: %w", err)
		}
		logger.audit = auditLog
	}

	// Create multi-writer
	var mw io.Writer
	if len(writers) > 1 {
		mw = io.MultiWriter(writers...)
	} else if len(writers) == 1 {
		mw = writers[0]
	} else {
		mw = io.Discard
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger.zl = zerolog.New(mw).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	logger.ctxLogger.logger = logger

	return logger, nil
}

// NewRotatableLogger creates a rotatable file logger with buffered I/O for performance
func newRotatableLogger(filename string, cfg Config) (*rotatableLogger, error) {
	rl := &rotatableLogger{
		filename: filename,
	}

	// Open file in append mode, create if doesn't exist
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	rl.file = file

	// Use buffered writer (64KB buffer) to reduce syscalls and improve I/O performance
	rl.bufWriter = bufio.NewWriterSize(file, 64*1024)
	rl.writer = rl.bufWriter

	return rl, nil
}

// createConsoleWriter creates the console writer based on configuration
func createConsoleWriter(cfg Config) io.Writer {
	if cfg.ConsolePretty {
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
		}
	}
	// Buffer console output too to reduce syscalls
	return bufio.NewWriterSize(os.Stdout, 32*1024)
}

// Init initializes the global logger with default configuration
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		defaultLogger, err = New(cfg)
	})
	return err
}

// InitDefault initializes the global logger with environment variables
func InitDefault() error {
	return Init(DefaultConfig())
}

// Get returns the global logger instance
func Get() *Logger {
	if defaultLogger == nil {
		// Create a default logger if not initialized
		defaultLogger, _ = New(DefaultConfig())
	}
	return defaultLogger
}

// With returns a context-aware logger
func With() *contextLogger {
	return Get().WithLogger()
}

// WithLogger returns a context-aware logger from a specific logger
func (l *Logger) WithLogger() *contextLogger {
	return l.ctxLogger
}

// Ctx returns a logger with context fields extracted from the context
func (cl *contextLogger) Ctx(ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return cl.logger.zl
	}

	zl := cl.logger.zl.With()

	// Extract common context fields
	if reqID, ok := ctx.Value(ContextKeyRequestID).(string); ok && reqID != "" {
		zl = zl.Str("request_id", reqID)
	}
	if userID, ok := ctx.Value(ContextKeyUserID).(string); ok && userID != "" {
		zl = zl.Str("user_id", userID)
	}
	if sessionID, ok := ctx.Value(ContextKeySessionID).(string); ok && sessionID != "" {
		zl = zl.Str("session_id", sessionID)
	}
	if tenantID, ok := ctx.Value(ContextKeyTenantID).(string); ok && tenantID != "" {
		zl = zl.Str("tenant_id", tenantID)
	}

	return zl.Logger()
}

// Debug logs a debug message
func (cl *contextLogger) Debug(ctx context.Context, msg string) {
	l := cl.Ctx(ctx)
	l.Debug().Msg(msg)
}

// Debugf logs a formatted debug message
func (cl *contextLogger) Debugf(ctx context.Context, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Debug().Msgf(format, args...)
}

// Info logs an info message
func (cl *contextLogger) Info(ctx context.Context, msg string) {
	l := cl.Ctx(ctx)
	l.Info().Msg(msg)
}

// Infof logs a formatted info message
func (cl *contextLogger) Infof(ctx context.Context, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Info().Msgf(format, args...)
}

// Warn logs a warning message
func (cl *contextLogger) Warn(ctx context.Context, msg string) {
	l := cl.Ctx(ctx)
	l.Warn().Msg(msg)
}

// Warnf logs a formatted warning message
func (cl *contextLogger) Warnf(ctx context.Context, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Warn().Msgf(format, args...)
}

// Error logs an error message
func (cl *contextLogger) Error(ctx context.Context, msg string) {
	l := cl.Ctx(ctx)
	l.Error().Msg(msg)
}

// Errorf logs a formatted error message
func (cl *contextLogger) Errorf(ctx context.Context, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Error().Msgf(format, args...)
}

// Fatal logs a fatal message and exits
func (cl *contextLogger) Fatal(ctx context.Context, msg string) {
	l := cl.Ctx(ctx)
	l.Fatal().Msg(msg)
}

// Fatalf logs a formatted fatal message and exits
func (cl *contextLogger) Fatalf(ctx context.Context, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Fatal().Msgf(format, args...)
}

// Err logs an error with the given error
func (cl *contextLogger) Err(ctx context.Context, err error, msg string) {
	l := cl.Ctx(ctx)
	l.Error().Err(err).Msg(msg)
}

// Errf logs a formatted error with the given error
func (cl *contextLogger) Errf(ctx context.Context, err error, format string, args ...interface{}) {
	l := cl.Ctx(ctx)
	l.Error().Err(err).Msgf(format, args...)
}

// Stack logs the stack trace
func (cl *contextLogger) Stack(ctx context.Context) *zerolog.Event {
	l := cl.Ctx(ctx)
	return l.Error().Stack()
}

// WithField returns a logger event with a field
func (cl *contextLogger) WithField(ctx context.Context, key string, value interface{}) *zerolog.Event {
	l := cl.Ctx(ctx)
	e := l.With().Interface(key, value)
	loggerBuilt := e.Logger()
	return loggerBuilt.Info()
}

// WithFields returns a logger event with multiple fields
func (cl *contextLogger) WithFields(ctx context.Context, fields map[string]interface{}) *zerolog.Event {
	l := cl.Ctx(ctx)
	e := l.With()
	for k, v := range fields {
		e = e.Interface(k, v)
	}
	builtLogger := e.Logger()
	return builtLogger.Info()
}

// WithError returns a logger event with an error
func (cl *contextLogger) WithError(ctx context.Context, err error) *zerolog.Event {
	l := cl.Ctx(ctx)
	return l.Error().Err(err)
}

// WithRequestID returns a context with the request ID set
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// WithUserID returns a context with the user ID set
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// WithSessionID returns a context with the session ID set
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ContextKeySessionID, sessionID)
}

// WithTenantID returns a context with the tenant ID set
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenantID, tenantID)
}

// GetRequestID returns the request ID from the context
func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return reqID
	}
	return ""
}

// GetUserID returns the user ID from the context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(ContextKeyUserID).(string); ok {
		return userID
	}
	return ""
}

// GetSessionID returns the session ID from the context
func GetSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value(ContextKeySessionID).(string); ok {
		return sessionID
	}
	return ""
}

// WriteLog writes a log entry to a specific log file
func (l *Logger) WriteLog(logType string, level string, msg string, fields map[string]interface{}) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var rl *rotatableLogger
	switch logType {
	case "all":
		rl = l.all
	case "error":
		rl = l.error
	case "access":
		rl = l.access
	case "audit":
		rl = l.audit
	default:
		rl = l.all
	}

	if rl == nil {
		return nil
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Use a sub-logger that writes to this rotatable file
	lvl := parseLevel(level)
	var buf bytes.Buffer
	subLogger := zerolog.New(&buf).Level(lvl).With().Timestamp().Logger()

	event := subLogger.Info()
	if lvl == zerolog.ErrorLevel {
		event = subLogger.Error()
	} else if lvl == zerolog.WarnLevel {
		event = subLogger.Warn()
	} else if lvl == zerolog.DebugLevel {
		event = subLogger.Debug()
	} else if lvl == zerolog.FatalLevel {
		event = subLogger.Fatal()
	}
	event.Msg(msg)

	// Add custom fields
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			event.Str(k, val)
		case int:
			event.Int(k, val)
		case int64:
			event.Int64(k, val)
		case float64:
			event.Float64(k, val)
		case bool:
			event.Bool(k, val)
		case error:
			event.AnErr(k, val)
		default:
			event.Interface(k, val)
		}
	}

	data := buf.Bytes()
	if len(data) == 0 {
		// Use a fresh logger and emit
		var newBuf bytes.Buffer
		freshLogger := zerolog.New(&newBuf).Level(lvl).With().Timestamp().Logger()
		freshEvent := freshLogger.Info()
		if lvl == zerolog.ErrorLevel {
			freshEvent = freshLogger.Error()
		} else if lvl == zerolog.WarnLevel {
			freshEvent = freshLogger.Warn()
		} else if lvl == zerolog.DebugLevel {
			freshEvent = freshLogger.Debug()
		}
		for k, v := range fields {
			switch val := v.(type) {
			case string:
				freshEvent.Str(k, val)
			case int:
				freshEvent.Int(k, val)
			case int64:
				freshEvent.Int64(k, val)
			case float64:
				freshEvent.Float64(k, val)
			case bool:
				freshEvent.Bool(k, val)
			case error:
				freshEvent.AnErr(k, val)
			default:
				freshEvent.Interface(k, val)
			}
		}
		freshEvent.Msg(msg)
		data = newBuf.Bytes()
	}

	_, err := rl.writer.Write(data)
	return err
}

// LogAccess writes an access log entry
func (l *Logger) LogAccess(reqID, method, path, clientIP, userAgent string, statusCode int, latency time.Duration, userID string) {
	fields := map[string]interface{}{
		"request_id":  reqID,
		"method":      method,
		"path":        path,
		"client_ip":   clientIP,
		"user_agent":  userAgent,
		"status":      statusCode,
		"latency_ms":  latency.Milliseconds(),
		"latency":     latency.String(),
		"user_id":     userID,
	}
	_ = l.WriteLog("access", "info", fmt.Sprintf("%s %s %d %s", method, path, statusCode, latency), fields)
}

// LogAudit writes an audit log entry
func (l *Logger) LogAudit(action, resource, userID, ipAddress, details string, success bool) {
	fields := map[string]interface{}{
		"action":   action,
		"resource": resource,
		"user_id":  userID,
		"ip":       ipAddress,
		"details":  details,
		"success":  success,
	}
	level := "info"
	if !success {
		level = "error"
	}
	_ = l.WriteLog("audit", level, fmt.Sprintf("AUDIT: %s on %s by %s", action, resource, userID), fields)
}

// LogError writes an error log entry
func (l *Logger) LogError(err error, msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["error"] = err.Error()
	fields["stack_trace"] = fmt.Sprintf("%+v", err)
	_ = l.WriteLog("error", "error", msg, fields)
}

// parseLevel converts a string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var errs []error

	// Helper to flush buffer and sync file
	flushAndSync := func(rl *rotatableLogger, name string) {
		if rl == nil || rl.file == nil {
			return
		}
		rl.mu.Lock()
		defer rl.mu.Unlock()
		// Flush buffer first if exists
		if rl.bufWriter != nil {
			if err := rl.bufWriter.Flush(); err != nil {
				errs = append(errs, fmt.Errorf("%s buffer flush: %w", name, err))
			}
		}
		// Then sync file to disk
		if err := rl.file.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("%s sync: %w", name, err))
		}
	}

	flushAndSync(l.all, "all.log")
	flushAndSync(l.error, "error.log")
	flushAndSync(l.access, "access.log")
	flushAndSync(l.audit, "audit.log")

	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %v", errs)
	}
	return nil
}

// Close closes the logger and all its file handles
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	// Flush buffer and close file
	closeLogger := func(rl *rotatableLogger, name string) {
		if rl == nil || rl.file == nil {
			return
		}
		if rl.bufWriter != nil {
			if err := rl.bufWriter.Flush(); err != nil {
				errs = append(errs, fmt.Errorf("%s flush: %w", name, err))
			}
		}
		if err := rl.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	closeLogger(l.all, "all.log")
	closeLogger(l.error, "error.log")
	closeLogger(l.access, "access.log")
	closeLogger(l.audit, "audit.log")

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// RotateLogFiles rotates all log files based on date
// This should be called periodically (e.g., daily) to implement daily rotation
func (l *Logger) RotateLogFiles() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	rotateFile := func(rl *rotatableLogger, filename string) error {
		if rl == nil || rl.file == nil {
			return nil
		}

		// Close current file
		if err := rl.file.Close(); err != nil {
			return err
		}

		// Generate new filename with date
		dateStr := time.Now().Format("2006-01-02")
		newFilename := fmt.Sprintf("%s.%s.log", 
			filepath.Join(filepath.Dir(filename), filepath.Base(filename[:len(filename)-4])), 
			dateStr)

		// Rename old file
		if _, err := os.Stat(filename); err == nil {
			if err := os.Rename(filename, newFilename); err != nil {
				return fmt.Errorf("failed to rename log file: %w", err)
			}
		}

		// Open new file
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		rl.file = file
		rl.writer = file

		return nil
	}

	if l.all != nil {
		if err := rotateFile(l.all, filepath.Join(l.config.LogDir, "all.log")); err != nil {
			errs = append(errs, fmt.Errorf("all.log rotation: %w", err))
		}
	}
	if l.error != nil {
		if err := rotateFile(l.error, filepath.Join(l.config.LogDir, "error.log")); err != nil {
			errs = append(errs, fmt.Errorf("error.log rotation: %w", err))
		}
	}
	if l.access != nil {
		if err := rotateFile(l.access, filepath.Join(l.config.LogDir, "access.log")); err != nil {
			errs = append(errs, fmt.Errorf("access.log rotation: %w", err))
		}
	}
	if l.audit != nil {
		if err := rotateFile(l.audit, filepath.Join(l.config.LogDir, "audit.log")); err != nil {
			errs = append(errs, fmt.Errorf("audit.log rotation: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rotation errors: %v", errs)
	}
	return nil
}

// CleanupOldLogs removes old log files beyond MaxBackups
func (l *Logger) CleanupOldLogs() error {
	if !l.config.EnableFileRotation || l.config.MaxBackups <= 0 {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var errs []error
	logFiles := []string{"all.log", "error.log", "access.log", "audit.log"}

	for _, baseName := range logFiles {
		pattern := filepath.Join(l.config.LogDir, baseName+".*.log")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("glob %s: %w", baseName, err))
			continue
		}

		// Sort by modification time
		sortByTime(matches)

		// Remove files beyond MaxBackups
		if len(matches) > l.config.MaxBackups {
			for _, file := range matches[l.config.MaxBackups:] {
				if err := os.Remove(file); err != nil {
					errs = append(errs, fmt.Errorf("remove %s: %w", file, err))
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// sortByTime sorts file paths by modification time (oldest first)
func sortByTime(paths []string) {
	// Simple bubble sort for small number of files
	for i := 0; i < len(paths)-1; i++ {
		for j := 0; j < len(paths)-i-1; j++ {
			info1, _ := os.Stat(paths[j])
			info2, _ := os.Stat(paths[j+1])
			if info1 != nil && info2 != nil && info1.ModTime().After(info2.ModTime()) {
				paths[j], paths[j+1] = paths[j+1], paths[j]
			}
		}
	}
}

// SetLevel dynamically changes the log level
func (l *Logger) SetLevel(level string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	l.zl = l.zl.Level(lvl)
	return nil
}

// Helper functions for direct logging (non-context-aware)

// Debug logs a debug message using the global logger
func Debug(msg string) {
	Get().ctxLogger.Debug(nil, msg)
}

// Debugf logs a formatted debug message using the global logger
func Debugf(format string, args ...interface{}) {
	Get().ctxLogger.Debugf(nil, format, args...)
}

// Info logs an info message using the global logger
func Info(msg string) {
	Get().ctxLogger.Info(nil, msg)
}

// Infof logs a formatted info message using the global logger
func Infof(format string, args ...interface{}) {
	Get().ctxLogger.Infof(nil, format, args...)
}

// Warn logs a warning message using the global logger
func Warn(msg string) {
	Get().ctxLogger.Warn(nil, msg)
}

// Warnf logs a formatted warning message using the global logger
func Warnf(format string, args ...interface{}) {
	Get().ctxLogger.Warnf(nil, format, args...)
}

// Error logs an error message using the global logger
func Error(msg string) {
	Get().ctxLogger.Error(nil, msg)
}

// Errorf logs a formatted error message using the global logger
func Errorf(format string, args ...interface{}) {
	Get().ctxLogger.Errorf(nil, format, args...)
}

// Fatal logs a fatal message and exits using the global logger
func Fatal(msg string) {
	Get().ctxLogger.Fatal(nil, msg)
}

// Fatalf logs a formatted fatal message and exits using the global logger
func Fatalf(format string, args ...interface{}) {
	Get().ctxLogger.Fatalf(nil, format, args...)
}

// Err logs an error using the global logger
func Err(err error, msg string) {
	Get().ctxLogger.Err(nil, err, msg)
}

// Logger exposure for direct zerolog access
func Z() zerolog.Logger {
	return Get().zl
}

// Package-level zerolog for convenience
func init() {
	// Initialize with defaults
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
}
