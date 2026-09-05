package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
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

	// Voice Call
	STUNServers []string

	// System Prompt
	SystemPrompt string

	// Asterisk AMI (Asterisk Manager Interface) - Production call center integration
	Asterisk AsteriskAMIConfig
	// Asterisk ARI (Asterisk REST Interface) - WebRTC Stasis app integration
	AsteriskARI AsteriskARIConfig
}

// AsteriskAMIConfig holds AMI connection settings for Asterisk PBX.
type AsteriskAMIConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	Context  string
	Trunk    string
	Queue    string
}

// AsteriskARIConfig holds ARI connection settings for Asterisk WebSocket/REST.
type AsteriskARIConfig struct {
	Enabled      bool
	BaseURL      string // e.g. "http://asterisk:8088" (HTTP, not ws://)
	WSURL        string // e.g. "ws://asterisk:8088/ari/events"
	Username     string
	Password     string
	AppName      string // Stasis app name, e.g. "dongdo-ivr"
	ReconnectSec int
}

// Load reads configuration from environment variables with sensible defaults.
// It first loads variables from .env file using dotenv, then falls back to OS environment variables.
func Load() *Config {
	// Load .env file if present (doesn't error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Println("[config] .env file not found, using environment variables only")
	} else {
		log.Println("[config] Loaded configuration from .env file")
	}

	return &Config{
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

		// Asterisk AMI
		Asterisk: AsteriskAMIConfig{
			Enabled:  getEnvBool("ASTERISK_ENABLED", false),
			Host:     getEnv("ASTERISK_HOST", "localhost"),
			Port:     getEnvInt("ASTERISK_PORT", 5038),
			Username: getEnv("ASTERISK_USER", "dongdo"),
			Password: getEnv("ASTERISK_PASS", ""),
			Context:  getEnv("ASTERISK_CONTEXT", "from-internal"),
			Trunk:    getEnv("ASTERISK_TRUNK", "PJSIP/trunk"),
			Queue:    getEnv("ASTERISK_QUEUE", "dongdo-queue"),
		},
		// Asterisk ARI (WebRTC / Stasis)
		AsteriskARI: AsteriskARIConfig{
			Enabled:      getEnvBool("ASTERISK_ARI_ENABLED", true),
			BaseURL:      getEnv("ASTERISK_ARI_BASE_URL", "http://asterisk:8088"),
			WSURL:        getEnv("ASTERISK_ARI_WS_URL", "ws://asterisk:8088/ari/events"),
			Username:     getEnv("ASTERISK_ARI_USER", "dongdo"),
			Password:     getEnv("ASTERISK_ARI_PASS", ""),
			AppName:      getEnv("ASTERISK_ARI_APP", "dongdo-ivr"),
			ReconnectSec: getEnvInt("ASTERISK_ARI_RECONNECT_SEC", 5),
		},
	}
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
