package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	ServerPort string
	ServerHost string

	// Bootstrap / First-time setup
	EnableBootstrap bool
	AdminPath       string

	// Database
	DatabaseURL string

	// Redis (Upstash)
	RedisURL string

	// Qdrant Vector DB
	QdrantHost string
	QdrantPort int

	// LLM Providers (Claude, OpenAI, Gemini)
	AnthropicAPIKey      string
	AnthropicWorkspaceID string
	OpenAIAPIKey         string
	GeminiAPIKey         string
	LLMModel             string
	LLMTemperature       float64
	LLMMaxTokens         int

	// Embedding
	EmbeddingModel string

	// RAG Parameters
	ChunkSize    int
	ChunkOverlap int
	RetrieverK   int
	MemoryWindow int

	// Paths
	DocumentsDir string

	// WebSocket
	WSPingInterval int // seconds
	WSWriteTimeout int // seconds

	// Workers
	DBBatchSize     int
	DBBatchInterval int // milliseconds
	RetryMaxCount   int
	RetryClaimAfter int // seconds

	// Voice Call (legacy STUN)
	STUNServers []string

	// System Prompt
	SystemPrompt string

	// Asterisk ARI
	ARI AsteriskConfig

	// Call Subsystem
	Call CallConfig
}

// AsteriskConfig bundles ARI connection settings.
type AsteriskConfig struct {
	URL                string
	User               string
	Password           string
	App                string
	RecordingEnabled   bool
	WSReconnectBackoff time.Duration
	HTTPTimeout        time.Duration
}

// CallConfig bundles call subsystem tunables.
type CallConfig struct {
	AgentRingTimeout time.Duration
	MaxDuration      time.Duration
	QueueTTL         time.Duration
	ReconcileEvery   time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		// Server
		ServerPort: getEnv("PORT", "8080"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),

		// Bootstrap / First-time setup
		EnableBootstrap: getEnvBool("ENABLE_BOOTSTRAP", false),
		AdminPath:       getEnv("ADMIN_PATH", "/admin"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://dongdo:dongdo@postgres:5432/dongdo_cs?sslmode=disable"),

		// Redis
		RedisURL: getEnv("REDIS_URL", ""),

		// Qdrant
		QdrantHost: getEnv("QDRANT_HOST", "localhost"),
		QdrantPort: getEnvInt("QDRANT_PORT", 6334),

		// Anthropic Claude
		AnthropicAPIKey:      getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicWorkspaceID: getEnv("ANTHROPIC_WORKSPACE_ID", ""),
		OpenAIAPIKey:         getEnv("OPENAI_API_KEY", ""),
		GeminiAPIKey:         getEnv("GEMINI_API_KEY", ""),
		LLMModel:             getEnv("LLM_MODEL", "claude-haiku-4-5-20251001"),
		LLMTemperature:       getEnvFloat("LLM_TEMPERATURE", 0.1),
		LLMMaxTokens:         getEnvInt("LLM_MAX_TOKENS", 4096),

		// Embedding
		EmbeddingModel: getEnv("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2"),

		// RAG
		ChunkSize:    getEnvInt("CHUNK_SIZE", 800),
		ChunkOverlap: getEnvInt("CHUNK_OVERLAP", 200),
		RetrieverK:   getEnvInt("RETRIEVER_K", 5),
		MemoryWindow: getEnvInt("MEMORY_WINDOW_SIZE", 10),

		// Paths
		DocumentsDir: getEnv("DOCUMENTS_DIR", "./tailieu"),

		// WebSocket
		WSPingInterval: getEnvInt("WS_PING_INTERVAL", 30),
		WSWriteTimeout: getEnvInt("WS_WRITE_TIMEOUT", 10),

		// Workers
		DBBatchSize:     getEnvInt("DB_BATCH_SIZE", 50),
		DBBatchInterval: getEnvInt("DB_BATCH_INTERVAL_MS", 2000),
		RetryMaxCount:   getEnvInt("RETRY_MAX_COUNT", 3),
		RetryClaimAfter: getEnvInt("RETRY_CLAIM_AFTER_SEC", 60),

		// Voice Call (Google free STUN servers)
		STUNServers: strings.Split(getEnv("STUN_SERVERS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"), ","),

		// System Prompt
		SystemPrompt: getEnv("SYSTEM_PROMPT", defaultSystemPrompt),

		// Asterisk ARI
		ARI: AsteriskConfig{
			URL:                getEnv("ASTERISK_ARI_URL", "http://asterisk:8088/ari"),
			User:               getEnv("ASTERISK_ARI_USER", "callservice"),
			Password:           getEnv("ASTERISK_ARI_PASSWORD", "callsecret"),
			App:                getEnv("ASTERISK_ARI_APP", "callapp"),
			RecordingEnabled:   getEnv("ASTERISK_RECORDING_ENABLED", "false") == "true",
			WSReconnectBackoff: getEnvDuration("ASTERISK_WS_BACKOFF", 2*time.Second),
			HTTPTimeout:        getEnvDuration("ASTERISK_HTTP_TIMEOUT", 5*time.Second),
		},

		// Call subsystem
		Call: CallConfig{
			AgentRingTimeout: getEnvDuration("CALL_AGENT_RING_TIMEOUT", 30*time.Second),
			MaxDuration:      getEnvDuration("CALL_MAX_DURATION", 30*time.Minute),
			QueueTTL:         getEnvDuration("CALL_QUEUE_TTL", 10*time.Minute),
			ReconcileEvery:   getEnvDuration("CALL_RECONCILE_INTERVAL", 30*time.Second),
		},
	}
}

func getEnvDuration(k string, fb time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fb
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return strings.ToLower(v) == "true" || v == "1"
}

const defaultSystemPrompt = `Bạn là chuyên viên Chăm sóc khách hàng của Đông Đô Partners.`
