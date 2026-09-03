package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type Client struct {
	apiKey      string
	workspaceID string
	openAIKey   string
	geminiKey   string
	model       string
	temp        float64
	httpClient  *http.Client
}

func NewClient(apiKey, workspaceID, openAIKey, geminiKey, model string, temperature float64) *Client {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &Client{
		apiKey:      apiKey,
		workspaceID: workspaceID,
		openAIKey:   openAIKey,
		geminiKey:   geminiKey,
		model:       model,
		temp:        temperature,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

type MessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	System      string        `json:"system,omitempty"`
	Messages    []MessageItem `json:"messages"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicResponse struct {
	Content []ContentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) GenerateResponse(
	ctx context.Context,
	systemPrompt string,
	history []*domain.Message,
	query string,
	contextBlock string,
) (reply string, isFallback bool, err error) {
	// 1. Construct system message with retrieved context
	fullSystemPrompt := systemPrompt
	if contextBlock != "" {
		fullSystemPrompt += fmt.Sprintf("\n\nDỮ LIỆU TỪ CƠ SỞ KIẾN THỨC (Knowledge Base):\n===\n%s\n===\nHãy dựa HOÀN TOÀN vào dữ liệu trên để trả lời câu hỏi của khách hàng. KHÔNG ĐƯỢC sử dụng bất kỳ kiến thức nào bên ngoài dữ liệu này.", contextBlock)
	} else {
		fullSystemPrompt += "\n\nKHÔNG TÌM THẤY DỮ LIỆU LIÊN QUAN trong Cơ sở kiến thức.\nHãy thực hiện đúng 2 bước: Xin lỗi + Chuyển giao chuyên viên CSKH với đúng nguyên văn câu chốt."
	}

	// 2. Build conversation history messages
	var messages []MessageItem
	for _, m := range history {
		if m.SenderType == domain.SenderGuest {
			messages = append(messages, MessageItem{Role: "user", Content: m.Content})
		} else if m.SenderType == domain.SenderAI || m.SenderType == domain.SenderHumanCS {
			messages = append(messages, MessageItem{Role: "assistant", Content: m.Content})
		}
	}

	// Append current user query
	messages = append(messages, MessageItem{Role: "user", Content: query})

	// 3. Try Anthropic Claude API if key is present
	if c.apiKey != "" {
		reqBody := AnthropicRequest{
			Model:       c.model,
			MaxTokens:   4096,
			Temperature: c.temp,
			System:      fullSystemPrompt,
			Messages:    messages,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err == nil {
			httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
			if err == nil {
				httpReq.Header.Set("x-api-key", c.apiKey)
				httpReq.Header.Set("anthropic-version", "2023-06-01")
				httpReq.Header.Set("content-type", "application/json")
				if c.workspaceID != "" {
					httpReq.Header.Set("anthropic-workspace-id", c.workspaceID)
				}

				resp, err := c.httpClient.Do(httpReq)
				if err == nil {
					defer resp.Body.Close()
					respBytes, _ := io.ReadAll(resp.Body)

					if resp.StatusCode == http.StatusOK {
						var anthropicResp AnthropicResponse
						if err := json.Unmarshal(respBytes, &anthropicResp); err != nil {
							log.Printf("⚠️ Anthropic JSON unmarshal error: %v | body: %s", err, string(respBytes))
						} else {
							var replyBuilder strings.Builder
							for _, block := range anthropicResp.Content {
								if block.Type == "text" {
									replyBuilder.WriteString(block.Text)
								}
							}
							reply = replyBuilder.String()
							fallbackPhrase := "chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay"
							isFallback = strings.Contains(strings.ToLower(reply), strings.ToLower(fallbackPhrase)) || contextBlock == ""

							// Only return if we have actual content; otherwise fall through to next provider.
							if reply != "" {
								return reply, isFallback, nil
							}
							log.Printf("⚠️ Anthropic 200 OK but empty text content | body: %s", string(respBytes))
						}
					} else {
						log.Printf("⚠️ Anthropic Claude API response status %d: %s", resp.StatusCode, string(respBytes))
					}
				}
			}
		}
	}

	// 4. Try Google Gemini API if key is present
	if c.geminiKey != "" {
		geminiReply, geminiErr := c.generateGemini(ctx, fullSystemPrompt, messages)
		if geminiErr == nil && geminiReply != "" {
			fallbackPhrase := "chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay"
			isFallback = strings.Contains(strings.ToLower(geminiReply), strings.ToLower(fallbackPhrase)) || contextBlock == ""
			return geminiReply, isFallback, nil
		}
	}

	// 5. Try OpenAI API if key is present
	if c.openAIKey != "" {
		openAIReply, openAIErr := c.generateOpenAI(ctx, fullSystemPrompt, messages)
		if openAIErr == nil && openAIReply != "" {
			fallbackPhrase := "chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay"
			isFallback = strings.Contains(strings.ToLower(openAIReply), strings.ToLower(fallbackPhrase)) || contextBlock == ""
			return openAIReply, isFallback, nil
		}
	}

	// 6. Intelligent Local Knowledge Synthesizer fallback if no cloud LLM responded
	if contextBlock != "" {
		synthesized, isFallbackLocal := synthesizeKnowledgeResponse(query, contextBlock)
		if synthesized != "" {
			return synthesized, isFallbackLocal, nil
		}
	}

	return "Dạ xin lỗi anh/chị, hiện tại em chưa có thông tin chi tiết về nội dung này trong hệ thống dữ liệu của Đông Đô Partners. Vui lòng đợi trong giây lát, chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay.", true, nil
}

func synthesizeKnowledgeResponse(query, contextBlock string) (string, bool) {
	chunks := strings.Split(contextBlock, "\n\n---\n\n")
	var bestChunk string

	// Look for chunks that are substantial paragraphs (not just table of contents headers)
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		lines := strings.Split(trimmed, "\n")
		// Filter out table of contents headers
		if len(lines) <= 2 && len(trimmed) < 60 {
			continue
		}
		// If chunk contains detailed explanation
		if len(trimmed) > 100 && !strings.HasPrefix(trimmed, "1. ") && !strings.HasPrefix(trimmed, "I. ") {
			bestChunk = trimmed
			break
		}
	}

	if bestChunk == "" && len(chunks) > 0 {
		for _, chunk := range chunks {
			trimmed := strings.TrimSpace(chunk)
			if len(trimmed) > 80 {
				bestChunk = trimmed
				break
			}
		}
	}

	if bestChunk != "" {
		formatted := fmt.Sprintf("Dạ, theo tài liệu hướng dẫn của Đông Đô Partners:\n\n%s\n\nNếu anh/chị cần giải đáp thêm chi tiết, chuyên viên CSKH sẵn sàng hỗ trợ tiếp cho anh/chị ạ! 😊", bestChunk)
		return formatted, false
	}

	return "Dạ xin lỗi anh/chị, hiện tại em chưa có thông tin chi tiết về nội dung này trong hệ thống dữ liệu của Đông Đô Partners. Vui lòng đợi trong giây lát, chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay.", true
}

func (c *Client) generateGemini(ctx context.Context, systemPrompt string, messages []MessageItem) (string, error) {
	type Part struct {
		Text string `json:"text"`
	}
	type Content struct {
		Role  string `json:"role"`
		Parts []Part `json:"parts"`
	}
	type GeminiReq struct {
		SystemInstruction *struct {
			Parts []Part `json:"parts"`
		} `json:"system_instruction,omitempty"`
		Contents []Content `json:"contents"`
	}

	var contents []Content
	for _, m := range messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, Content{
			Role:  role,
			Parts: []Part{{Text: m.Content}},
		})
	}

	reqBody := GeminiReq{
		SystemInstruction: &struct {
			Parts []Part `json:"parts"`
		}{
			Parts: []Part{{Text: systemPrompt}},
		},
		Contents: contents,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", c.geminiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini error %d: %s", resp.StatusCode, string(respBytes))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("empty gemini candidates")
}

func (c *Client) generateOpenAI(ctx context.Context, systemPrompt string, messages []MessageItem) (string, error) {
	type OpenAIMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var oaiMessages []OpenAIMessage
	oaiMessages = append(oaiMessages, OpenAIMessage{Role: "system", Content: systemPrompt})
	for _, m := range messages {
		oaiMessages = append(oaiMessages, OpenAIMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := map[string]interface{}{
		"model":       "gpt-4o-mini",
		"temperature": c.temp,
		"messages":    oaiMessages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.openAIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBytes))
	}

	var oaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBytes, &oaiResp); err != nil {
		return "", err
	}

	if len(oaiResp.Choices) > 0 {
		return oaiResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("empty openai choices")
}
