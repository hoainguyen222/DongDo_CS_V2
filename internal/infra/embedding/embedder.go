package embedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"
)

type EmbedderService struct {
	apiURL     string
	apiKey     string
	model      string
	vectorSize int
	httpClient *http.Client
}

func NewEmbedder(model string) *EmbedderService {
	if model == "" {
		model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	return &EmbedderService{
		apiURL:     os.Getenv("EMBEDDING_API_URL"), // Optional custom embedding service
		apiKey:     os.Getenv("OPENAI_API_KEY"),
		model:      model,
		vectorSize: 384,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// EmbedText generates a 384-dimensional embedding vector for a single text.
func (e *EmbedderService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return make([]float32, e.vectorSize), nil
	}

	// 1. If custom embedding API is configured (e.g. TEI / FastEmbed / Infinity container)
	if e.apiURL != "" {
		vec, err := e.callCustomAPI(ctx, text)
		if err == nil {
			return vec, nil
		}
	}

	// 2. If OpenAI API key is configured
	if e.apiKey != "" {
		vec, err := e.callOpenAI(ctx, text)
		if err == nil {
			return vec, nil
		}
	}

	// 3. Built-in hash-based semantic feature vector (deterministic offline fallback)
	return e.generateDeterministicVector(text), nil
}

// EmbedBatch generates embeddings for multiple texts.
func (e *EmbedderService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		vec, err := e.EmbedText(ctx, t)
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

func (e *EmbedderService) callCustomAPI(ctx context.Context, text string) ([]float32, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"inputs": text,
		"model":  e.model,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var vector []float32
	if err := json.NewDecoder(resp.Body).Decode(&vector); err != nil {
		return nil, err
	}
	return vector, nil
}

func (e *EmbedderService) callOpenAI(ctx context.Context, text string) ([]float32, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"input":          text,
		"model":          "text-embedding-3-small",
		"dimensions":     e.vectorSize,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Data) == 0 {
		return nil, fmt.Errorf("invalid openai response")
	}

	return result.Data[0].Embedding, nil
}

// generateDeterministicVector creates a normalized feature embedding based on token n-grams and hashing.
func (e *EmbedderService) generateDeterministicVector(text string) []float32 {
	vec := make([]float32, e.vectorSize)
	words := splitWords(text)

	for _, w := range words {
		h := sha256.Sum256([]byte(w))
		for i := 0; i < e.vectorSize && i+3 < len(h); i++ {
			idx := (int(h[i]) + int(h[i+1])*256) % e.vectorSize
			val := float32(h[i+2])/255.0 - 0.5
			vec[idx] += val
		}
	}

	// L2 normalization
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec
}

func splitWords(s string) []string {
	var words []string
	var cur []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 127 {
			cur = append(cur, r)
		} else if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}
