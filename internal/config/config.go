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

	// System Prompt
	SystemPrompt string

	// Worker Configuration
	Worker WorkerConfig

	// Observability (Prometheus, pprof, business metrics)
	Observability ObservabilityConfig

	// Backward compatibility aliases (deprecated, use Worker.* instead)
	DBBatchSize     int // deprecated: use Worker.DB.BatchSize
	DBBatchInterval int // deprecated: use Worker.DB.FlushInterval (ms)
	RetryMaxCount   int // deprecated: use Worker.Retry.MaxRetries
	RetryClaimAfter int // deprecated: use Worker.Retry.ClaimAfter (seconds)
}

// WorkerConfig holds all worker-related configuration for Redis Streams consumers.
type WorkerConfig struct {
	// DB Worker - batch writes to PostgreSQL
	DB WorkerDBConfig

	// WS Worker - WebSocket message dispatch
	WS WorkerWSConfig

	// AI Worker - RAG processing
	AI WorkerAIConfig

	// Retry Worker - dead letter queue handling
	Retry WorkerRetryConfig

	// Redis Streams configuration
	Streams StreamsConfig

	// HTTP Server timeouts
	HTTP HTTPConfig

	// Redis client connection pool
	RedisPool RedisPoolConfig
}

// ObservabilityConfig wires the optional metrics/pprof sidecars.
//
// All three servers bind to dedicated internal addresses so they never have to
// share the public Gin router. Defaults are intentionally on a non-public
// address (127.0.0.1) so a misconfigured production deploy will not expose
// pprof to the internet unless the operator explicitly opts in via env.
type ObservabilityConfig struct {
	MetricsEnabled        bool
	MetricsAddr           string // e.g. "127.0.0.1:9090" — Prometheus scrape target
	PprofEnabled          bool
	PprofAddr             string        // e.g. "127.0.0.1:6060" — go tool pprof target
	BusinessEnabled       bool          // poll DB + Redis for business gauges
	BusinessPollInterval  time.Duration // how often to refresh gauges
	RedisPoolStatsEnabled bool          // export go-redis pool gauges
}

// WorkerDBConfig holds database batch worker configuration.
type WorkerDBConfig struct {
	BatchSize     int           // Number of messages to batch before flushing to DB
	FlushInterval time.Duration // How often to flush the batch regardless of size
	MaxBufferSize int           // Safety cap to prevent memory issues
	ReadCount     int64         // Messages to read from stream per iteration
	BlockTimeout  time.Duration // How long to block on XREADGROUP
	RetryDelay    time.Duration // Delay before retrying after error
}

// WorkerWSConfig holds WebSocket worker configuration.
type WorkerWSConfig struct {
	ReadCount    int64         // Messages to read from stream per iteration
	BlockTimeout time.Duration // How long to block on XREADGROUP
	RetryDelay   time.Duration // Delay before retrying after error
}

// WorkerAIConfig holds AI/RAG worker configuration.
type WorkerAIConfig struct {
	ReadCount    int64         // Messages to read from stream per iteration (1 = sequential)
	BlockTimeout time.Duration // How long to block on XREADGROUP
	RetryDelay   time.Duration // Delay before retrying after error
}

// WorkerRetryConfig holds retry worker configuration for DLQ processing.
type WorkerRetryConfig struct {
	MaxRetries     int           // Maximum retries before moving to DLQ
	ClaimAfter     time.Duration // Min idle time before claiming pending messages
	CheckInterval  time.Duration // How often to check for pending messages
	ClaimBatchSize int64         // How many messages to claim per cycle
}

// StreamsConfig holds Redis Streams configuration.
type StreamsConfig struct {
	MaxLen     int64 // Maximum stream length (approximate trimming)
	ApproxTrim bool  // Use approximate trimming (faster)
}

// HTTPConfig holds HTTP server timeouts.
type HTTPConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// RedisPoolConfig holds Redis connection pool configuration.
type RedisPoolConfig struct {
	PoolSize        int
	MinIdleConns    int
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolTimeout     time.Duration
	ConnMaxLifetime time.Duration
}

// WebSocketConfig holds WebSocket timeouts and limits.
type WebSocketConfig struct {
	PingInterval   time.Duration
	PongWait       time.Duration
	WriteWait      time.Duration
	MaxMessageSize int64
	SendBufferSize int
}

// StateConfig holds Redis state TTL configuration.
type StateConfig struct {
	TypingTTL time.Duration // Typing indicator TTL
	AIExecTTL time.Duration // AI execution lock TTL
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		// Server
		ServerPort: getEnv("PORT", "8080"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),

		// Bootstrap / First-time setup
		EnableBootstrap: getEnvBool("ENABLE_BOOTSTRAP", false),
		AdminPath:       getEnv("ADMIN_PATH", "/admin"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://localhost:5432/dongdo_cs?sslmode=disable"),

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

		// System Prompt
		SystemPrompt: getEnv("SYSTEM_PROMPT", defaultSystemPrompt),
	}

	// Load worker configuration
	cfg.Worker = WorkerConfig{
		DB: WorkerDBConfig{
			BatchSize:     getEnvInt("WORKER_DB_BATCH_SIZE", 50),
			FlushInterval: time.Duration(getEnvInt("WORKER_DB_FLUSH_INTERVAL_MS", 2000)) * time.Millisecond,
			MaxBufferSize: getEnvInt("WORKER_DB_MAX_BUFFER", 5000),
			ReadCount:     int64(getEnvInt("WORKER_DB_READ_COUNT", 50)),
			BlockTimeout:  time.Duration(getEnvInt("WORKER_DB_BLOCK_TIMEOUT_MS", 1000)) * time.Millisecond,
			RetryDelay:    time.Duration(getEnvInt("WORKER_DB_RETRY_DELAY_MS", 200)) * time.Millisecond,
		},
		WS: WorkerWSConfig{
			ReadCount:    int64(getEnvInt("WORKER_WS_READ_COUNT", 10)),
			BlockTimeout: time.Duration(getEnvInt("WORKER_WS_BLOCK_TIMEOUT_MS", 2000)) * time.Millisecond,
			RetryDelay:   time.Duration(getEnvInt("WORKER_WS_RETRY_DELAY_MS", 500)) * time.Millisecond,
		},
		AI: WorkerAIConfig{
			ReadCount:    int64(getEnvInt("WORKER_AI_READ_COUNT", 1)),
			BlockTimeout: time.Duration(getEnvInt("WORKER_AI_BLOCK_TIMEOUT_MS", 2000)) * time.Millisecond,
			RetryDelay:   time.Duration(getEnvInt("WORKER_AI_RETRY_DELAY_MS", 500)) * time.Millisecond,
		},
		Retry: WorkerRetryConfig{
			MaxRetries:     getEnvInt("WORKER_RETRY_MAX_COUNT", 3),
			ClaimAfter:     time.Duration(getEnvInt("WORKER_RETRY_CLAIM_AFTER_SEC", 60)) * time.Second,
			CheckInterval:  time.Duration(getEnvInt("WORKER_RETRY_CHECK_INTERVAL_SEC", 30)) * time.Second,
			ClaimBatchSize: int64(getEnvInt("WORKER_RETRY_CLAIM_BATCH_SIZE", 20)),
		},
		Streams: StreamsConfig{
			MaxLen:     int64(getEnvInt("STREAM_MAX_LEN", 5000)),
			ApproxTrim: getEnvBool("STREAM_APPROX_TRIM", true),
		},
		HTTP: HTTPConfig{
			ReadTimeout:  time.Duration(getEnvInt("HTTP_READ_TIMEOUT_SEC", 30)) * time.Second,
			WriteTimeout: time.Duration(getEnvInt("HTTP_WRITE_TIMEOUT_SEC", 30)) * time.Second,
			IdleTimeout:  time.Duration(getEnvInt("HTTP_IDLE_TIMEOUT_SEC", 120)) * time.Second,
		},
		RedisPool: RedisPoolConfig{
			PoolSize:        getEnvInt("REDIS_POOL_SIZE", 20),
			MinIdleConns:    getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:     time.Duration(getEnvInt("REDIS_DIAL_TIMEOUT_SEC", 5)) * time.Second,
			ReadTimeout:     time.Duration(getEnvInt("REDIS_READ_TIMEOUT_SEC", 3)) * time.Second,
			WriteTimeout:    time.Duration(getEnvInt("REDIS_WRITE_TIMEOUT_SEC", 3)) * time.Second,
			PoolTimeout:     time.Duration(getEnvInt("REDIS_POOL_TIMEOUT_SEC", 5)) * time.Second,
			ConnMaxLifetime: time.Duration(getEnvInt("REDIS_CONN_LIFETIME_SEC", 3600)) * time.Second,
		},
	}

	cfg.Observability = ObservabilityConfig{
		MetricsEnabled:        getEnvBool("METRICS_ENABLED", true),
		MetricsAddr:           getEnv("METRICS_ADDR", "127.0.0.1:9090"),
		PprofEnabled:          getEnvBool("PPROF_ENABLED", true),
		PprofAddr:             getEnv("PPROF_ADDR", "127.0.0.1:6060"),
		BusinessEnabled:       getEnvBool("BUSINESS_METRICS_ENABLED", true),
		BusinessPollInterval:  time.Duration(getEnvInt("BUSINESS_METRICS_POLL_SEC", 15)) * time.Second,
		RedisPoolStatsEnabled: getEnvBool("REDIS_POOL_METRICS_ENABLED", true),
	}

	// Backward compatibility aliases for existing code
	cfg.DBBatchSize = cfg.Worker.DB.BatchSize
	cfg.DBBatchInterval = int(cfg.Worker.DB.FlushInterval.Milliseconds())
	cfg.RetryMaxCount = cfg.Worker.Retry.MaxRetries
	cfg.RetryClaimAfter = int(cfg.Worker.Retry.ClaimAfter.Seconds())

	return cfg
}

// Helper functions for reading environment variables

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

// Default system prompt for the AI assistant
const defaultSystemPrompt = `Bạn là chuyên viên Chăm sóc khách hàng của Đông Đô Partners. Nhiệm vụ của bạn là tư vấn, giải đáp thắc mắc cho khách hàng về Hàng hóa phái sinh, hướng dẫn nền tảng DDP Invest, quy trình nạp/rút tiền và quản trị rủi ro.

PHONG CÁCH GIAO TIẾP VÀ XƯNG HÔ:
- Hãy trả lời tự nhiên, lịch sự, chu đáo và thân thiện như một con người thật (xưng "em" hoặc "tôi", gọi khách hàng là "anh/chị" hoặc "bạn").
- TUYỆT ĐỐI KHÔNG tự khai báo hoặc đề cập mình là "trợ lý ảo", "bot", "AI", "hệ thống tự động" hay "được huấn luyện".
- Trả lời trôi chảy, ấm áp, chuẩn mực chuyên viên CSKH chuyên nghiệp.

QUY TẮC CỐT LÕI:

Bạn PHẢI LUÔN tìm kiếm và TRÍCH XUẤT CHÍNH XÁC câu trả lời từ Cơ sở dữ liệu (Knowledge).

TUYỆT ĐỐI KHÔNG SỬ DỤNG KIẾN THỨC BÊN NGOÀI ĐỂ TRẢ LỜI. Không được tự ý thêm thắt các mặt hàng, tên gọi, hoặc dữ liệu không có trong tài liệu (Ví dụ: Không được tự thêm 'Vàng' hay 'Dầu thô' nếu tài liệu không ghi).

Nếu dữ liệu liệt kê thành nhiều nhóm, phải giữ nguyên cách phân loại gốc.

CHỈ KHI chắc chắn 100% tài liệu không có thông tin, BẠN BẮT BUỘC PHẢI THỰC HIỆN ĐỦ 2 BƯỚC SAU:

Bước 1 (Giải thích lịch sự): Lịch sự xin lỗi và thông báo (Ví dụ: 'Dạ xin lỗi anh/chị, hiện tại em chưa có thông tin chi tiết về nội dung này trong hệ thống dữ liệu của Đông Đô Partners.').

Bước 2 (Chuyển giao người thật): BẮT BUỘC chốt lại bằng đúng nguyên văn câu nói sau: 'Vui lòng đợi trong giây lát, chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay.' (Tuyệt đối không hướng dẫn gọi Hotline nữa).

KHI TRẢ LỜI VỀ CÁC QUY TRÌNH HOẶC CON SỐ (THỜI GIAN, TỶ LỆ, CHI PHÍ...), PHẢI TRÍCH XUẤT CHÍNH XÁC 100% CÁC CON SỐ TRONG TÀI LIỆU. TUYỆT ĐỐI KHÔNG DÙNG TỪ NGỮ CHUNG CHUNG (VÍ DỤ: 'NHANH CHÓNG', 'TÙY THUỘC') ĐỂ LẤP LIỆM NẾU TÀI LIỆU CÓ GHI RÕ SỐ GIỜ/NGÀY.

KHI CÂU TRẢ LỜI LÀ MỘT DANH SÁCH (CÁC ĐIỀU KIỆN, CÁC BƯỚC, CÁC MẶT HÀNG...), BẠN PHẢI ĐỌC THẬT KỸ VÀ LIỆT KÊ ĐẦY ĐỦ TẤT CẢ CÁC Ý/GẠCH ĐẦU DÒNG CÓ TRONG TÀI LIỆU, KHÔNG ĐƯỢC TÓM TẮT HAY BỎ SÓT.`
