package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	ServerPort string
	ServerHost string

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
	ChunkSize      int
	ChunkOverlap   int
	RetrieverK     int
	MemoryWindow   int

	// Paths
	DocumentsDir string

	// WebSocket
	WSPingInterval  int      // seconds
	WSWriteTimeout  int      // seconds
	WSAllowedOrigins []string // whitelist for WS origin checks
	WSAdminInboxSession string // session ID for staff admin inbox (default "admin_inbox")

	// CORS
	CORSAllowedOrigins []string // whitelist for HTTP CORS headers

	// Rate Limiting (requests per minute per IP)
	RateLimitLoginRequestsPerMinute int // default 5
	RateLimitChatRequestsPerMinute  int // default 30
	RateLimitAdminRequestsPerMinute int // default 100
	RateLimitUploadRequestsPerMinute int // default 10

	// Workers
	DBBatchSize     int
	DBBatchInterval int // milliseconds
	RetryMaxCount   int
	RetryClaimAfter int // seconds

	// Voice Call
	STUNServers []string

	// System Prompt
	SystemPrompt string

	// JWT (Staff auth only)
	JWTManager   *security.StaffJWTManager
	JWTAccessTTL  time.Duration // default 15 minutes
	JWTRefreshTTL time.Duration // default 7 days
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		// Server
		ServerPort: getEnv("PORT", "8080"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),

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

		// WebSocket
		WSPingInterval: getEnvInt("WS_PING_INTERVAL", 30),
		WSWriteTimeout: getEnvInt("WS_WRITE_TIMEOUT", 10),

		// WebSocket security
		WSAllowedOrigins:    strings.Split(getEnv("WS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001"), ","),
		WSAdminInboxSession: getEnv("WS_ADMIN_INBOX_SESSION", "admin_inbox"),

		// CORS
		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001"), ","),

		// Rate Limiting (requests per minute per IP)
		RateLimitLoginRequestsPerMinute:  getEnvInt("RATE_LIMIT_LOGIN", 5),
		RateLimitChatRequestsPerMinute:   getEnvInt("RATE_LIMIT_CHAT", 30),
		RateLimitAdminRequestsPerMinute:  getEnvInt("RATE_LIMIT_ADMIN", 100),
		RateLimitUploadRequestsPerMinute: getEnvInt("RATE_LIMIT_UPLOAD", 10),

		// Workers
		DBBatchSize:     getEnvInt("DB_BATCH_SIZE", 50),
		DBBatchInterval: getEnvInt("DB_BATCH_INTERVAL_MS", 2000),
		RetryMaxCount:   getEnvInt("RETRY_MAX_COUNT", 3),
		RetryClaimAfter: getEnvInt("RETRY_CLAIM_AFTER_SEC", 60),

		// Voice Call (Google free STUN servers)
		STUNServers: strings.Split(getEnv("STUN_SERVERS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"), ","),

		// System Prompt
		SystemPrompt: getEnv("SYSTEM_PROMPT", defaultSystemPrompt),

		// JWT TTL defaults (manager initialized separately via LoadJWT)
		JWTAccessTTL:  time.Duration(getEnvInt("JWT_ACCESS_TTL_MINUTES", 15)) * time.Minute,
		JWTRefreshTTL: time.Duration(getEnvInt("JWT_REFRESH_TTL_HOURS", 168)) * time.Hour,
	}
}

// LoadJWT initializes the JWT manager from JWT_SECRET env var.
// Call this after Load() if you need JWT auth.
// Returns error if JWT_SECRET is missing or too short (< 32 chars).
func (cfg *Config) LoadJWT() error {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required for staff auth")
	}
	manager, err := security.NewStaffJWTManager(jwtSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	if err != nil {
		return fmt.Errorf("JWT config: %w", err)
	}
	cfg.JWTManager = manager
	return nil
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

KHI TRẢ LỜI VỀ CÁC QUY TRÌNH HOẶC CON SỐ (THỜI GIAN, TỶ LỆ, CHI PHÍ...), PHẢI TRÍCH XUẤT CHÍNH XÁC 100% CÁC CON SỐ TRONG TÀI LIỆU. TUYỆT ĐỐI KHÔNG DÙNG TỪ NGỮ CHUNG CHUNG (VÍ DỤ: 'NHANH CHÓNG', 'TÙY THUỘC') ĐỂ LẤP LIẾM NẾU TÀI LIỆU CÓ GHI RÕ SỐ GIỜ/NGÀY.

KHI CÂU TRẢ LỜI LÀ MỘT DANH SÁCH (CÁC ĐIỀU KIỆN, CÁC BƯỚC, CÁC MẶT HÀNG...), BẠN PHẢI ĐỌC THẬT KỸ VÀ LIỆT KÊ ĐẦY ĐỦ TẤT CẢ CÁC Ý/GẠCH ĐẦU DÒNG CÓ TRONG TÀI LIỆU, KHÔNG ĐƯỢC TÓM TẮT HAY BỎ SÓT.`
