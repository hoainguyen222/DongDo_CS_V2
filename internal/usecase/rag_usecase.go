package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraClaude "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/claude"
)

type RAGUseCase struct {
	vectorStore   domain.VectorStore
	embedder      domain.Embedder
	claudeClient  *infraClaude.Client
	messageRepo   domain.MessageRepository
	settingRepo   domain.SettingRepository
	defaultPrompt string
	memoryWindow  int
	retrieverK    int
}

func NewRAGUseCase(
	vectorStore domain.VectorStore,
	embedder domain.Embedder,
	claudeClient *infraClaude.Client,
	messageRepo domain.MessageRepository,
	settingRepo domain.SettingRepository,
	defaultPrompt string,
	memoryWindow int,
	retrieverK int,
) *RAGUseCase {
	if memoryWindow <= 0 {
		memoryWindow = 10
	}
	if retrieverK <= 0 {
		retrieverK = 5
	}
	return &RAGUseCase{
		vectorStore:   vectorStore,
		embedder:      embedder,
		claudeClient:  claudeClient,
		messageRepo:   messageRepo,
		settingRepo:   settingRepo,
		defaultPrompt: defaultPrompt,
		memoryWindow:  memoryWindow,
		retrieverK:    retrieverK,
	}
}

func isGreetingQuery(q string) bool {
	clean := strings.ToLower(strings.TrimSpace(q))
	clean = strings.Trim(clean, ".!?,~@#$%^&*()_+=-/\\")
	greetings := []string{
		"xin chào", "chào", "chào bạn", "chào em", "chào anh", "chào chị", "chào shop", "chào ad", "chào admin",
		"hi", "hello", "alo", "helo", "hey", "hế lô", "bắt đầu", "start",
		"tư vấn", "tư vấn giúp", "tư vấn giúp mình", "tư vấn giúp em", "tư vấn cho tôi", "tư vấn cho mình",
		"có ai ở đây không", "có ai online không", "có ai trực không", "giúp tôi", "hỗ trợ tôi", "hỗ trợ mình",
	}
	for _, g := range greetings {
		if clean == g || strings.HasPrefix(clean, g+" ") || strings.HasSuffix(clean, " "+g) {
			return true
		}
	}
	return false
}

// GenerateResponse executes the full RAG pipeline: Embedding -> Vector Search -> Prompt Building -> Claude Generation.
func (uc *RAGUseCase) GenerateResponse(ctx context.Context, sessionID, query string) (reply string, sources []string, isFallback bool, err error) {
	// 0. Natural Greeting & Intent Handling
	if isGreetingQuery(query) {
		reply = "Dạ, xin chào anh/chị! 👋\n\nEm là Trợ lý AI Chăm sóc khách hàng của Đông Đô Partners. Em rất vui được hỗ trợ anh/chị hôm nay!\n\nEm có thể giải đáp nhanh về:\n- 📈 **Hàng hóa phái sinh** (Khái niệm, đòn bẩy, lệnh giao dịch, ký quỹ)\n- 📱 **Nền tảng DDP Invest** (Hướng dẫn mở tài khoản, nạp rút tiền, biểu phí)\n- 🛡️ **Quản trị rủi ro & Xử lý sự cố**\n\nAnh/chị đang quan tâm đến nội dung nào để em hỗ trợ chi tiết ạ? 😊"
		return reply, nil, false, nil
	}

	// 1. Retrieve relevant knowledge chunks from Qdrant with confidence threshold
	var contextParts []string
	sourceSet := make(map[string]bool)

	if uc.embedder != nil && uc.vectorStore != nil {
		queryVec, err := uc.embedder.EmbedText(ctx, query)
		if err == nil {
			// Require minimum similarity score of 0.35 to avoid matching random documents on short queries
			docs, err := uc.vectorStore.Search(ctx, queryVec, uc.retrieverK, 0.35)
			if err == nil {
				for _, doc := range docs {
					if doc.Content != "" {
						contextParts = append(contextParts, doc.Content)
						if doc.Source != "" {
							sourceSet[doc.Source] = true
						}
					}
				}
			}
		}
	}

	contextBlock := strings.Join(contextParts, "\n\n---\n\n")

	for s := range sourceSet {
		sources = append(sources, s)
	}

	// 2. Fetch conversation history
	history, err := uc.messageRepo.GetHistory(ctx, sessionID)
	if err != nil {
		history = []*domain.Message{}
	}

	// Limit history to memory window (last N messages)
	maxHistory := uc.memoryWindow * 2
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	// 3. Get system prompt from settings or default
	systemPrompt, _ := uc.settingRepo.Get(ctx, "system_prompt", uc.defaultPrompt)

	// 4. Generate response via LLM Client
	reply, isFallback, err = uc.claudeClient.GenerateResponse(ctx, systemPrompt, history, query, contextBlock)
	if err != nil {
		return "", sources, true, fmt.Errorf("rag generation failed: %w", err)
	}

	lowerReply := strings.ToLower(reply)
	if contextBlock == "" ||
		strings.Contains(lowerReply, "chuyên viên cskh") ||
		strings.Contains(lowerReply, "chưa có thông tin") ||
		strings.Contains(lowerReply, "tham gia cuộc trò chuyện") {
		isFallback = true
	}

	return reply, sources, isFallback, nil
}
