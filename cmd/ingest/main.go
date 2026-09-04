package main

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraEmbedding "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/embedding"
	infraQdrant "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/qdrant"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	serviceName    = "dongdo-cs-ingest"
	serviceVersion = "2.0.0"
)

func init() {
	// Configure zerolog for production
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// Initialize logger with structured fields
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Str("service", serviceName).
		Str("version", serviceVersion).
		Logger()

	ctx := context.Background()

	cfg := config.Load()

	logger.Info().
		Str("service", serviceName).
		Str("version", serviceVersion).
		Str("qdrant_host", cfg.QdrantHost).
		Str("documents_dir", cfg.DocumentsDir).
		Msg("Document ingestion pipeline starting")

	startTime := time.Now()

	// 1. Connect to Qdrant
	qdrantClient, err := infraQdrant.NewClient(ctx, cfg.QdrantHost, cfg.QdrantPort, 384)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed to connect to Qdrant")
	}

	logger.Info().
		Str("host", cfg.QdrantHost).
		Int("port", cfg.QdrantPort).
		Msg("Connected to Qdrant")

	// Register cleanup
	go func() {
		<-ctx.Done()
		if err := qdrantClient.Close(); err != nil {
			logger.Error().
				Err(err).
				Msg("Error closing Qdrant connection")
		}
	}()

	// 2. Initialize Embedder
	embedder := infraEmbedding.NewEmbedder(cfg.EmbeddingModel)

	// 3. Find .docx files
	docsDir := cfg.DocumentsDir
	files, err := filepath.Glob(filepath.Join(docsDir, "*.docx"))
	if err != nil {
		logger.Error().
			Str("directory", docsDir).
			Err(err).
			Msg("Error scanning directory")
		return
	}

	if len(files) == 0 {
		logger.Warn().
			Str("directory", docsDir).
			Msg("No .docx files found in directory")
		return
	}

	logger.Info().
		Str("directory", docsDir).
		Int("file_count", len(files)).
		Msg("Found documents to ingest")

	totalChunks := 0
	totalChars := 0
	totalErrors := 0
	processedFiles := 0

	for _, fpath := range files {
		fname := filepath.Base(fpath)

		text, err := extractTextFromDocx(fpath)
		if err != nil {
			logger.Error().
				Str("file", fname).
				Err(err).
				Msg("Failed to read document")
			totalErrors++
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" {
			logger.Warn().
				Str("file", fname).
				Msg("Document is empty")
			totalErrors++
			continue
		}

		charCount := len(text)
		totalChars += charCount

		// Split text into chunks
		chunks := splitText(text, cfg.ChunkSize, cfg.ChunkOverlap)
		chunkCount := len(chunks)

		// Create knowledge documents with metadata
		docs := make([]*domain.KnowledgeDocument, len(chunks))
		for i, chunk := range chunks {
			docs[i] = &domain.KnowledgeDocument{
				ID:      uuid.New().String(),
				Content: chunk,
				Source:  fname,
				Metadata: map[string]interface{}{
					"source":       fname,
					"chunk_id":     fmt.Sprintf("%d", i),
					"total_chunks": fmt.Sprintf("%d", chunkCount),
					"ingested_at":  time.Now().Format(time.RFC3339),
					"type":         "knowledge_base",
				},
			}
		}

		// Generate embeddings
		vectors, err := embedder.EmbedBatch(ctx, chunks)
		if err != nil {
			logger.Error().
				Str("file", fname).
				Err(err).
				Msg("Embedding failed")
			totalErrors++
			continue
		}

		// Upsert to Qdrant
		err = qdrantClient.Upsert(ctx, docs, vectors)
		if err != nil {
			logger.Error().
				Str("file", fname).
				Err(err).
				Msg("Qdrant upsert failed")
			totalErrors++
			continue
		}

		totalChunks += chunkCount
		processedFiles++
	}

	// Summary
	logger.Info().
		Int("files_processed", processedFiles).
		Int("total_files", len(files)).
		Int("total_chunks", totalChunks).
		Int("total_characters", totalChars).
		Int("error_count", totalErrors).
		Dur("total_duration_ms", time.Since(startTime)).
		Msg("Ingest complete")
}

// extractTextFromDocx extracts all paragraphs from a .docx file (word/document.xml in zip).
func extractTextFromDocx(filepath string) (string, error) {
	r, err := zip.OpenReader(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open docx file: %w", err)
	}
	defer r.Close()

	var docXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open document.xml: %w", err)
			}
			break
		}
	}

	if docXML == nil {
		return "", fmt.Errorf("word/document.xml not found in %s", filepath)
	}
	defer docXML.Close()

	decoder := xml.NewDecoder(docXML)
	var textBuilder strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch elem := tok.(type) {
		case xml.CharData:
			textBuilder.Write(elem)
		case xml.StartElement:
			if elem.Name.Local == "p" || elem.Name.Local == "tr" {
				textBuilder.WriteString("\n")
			} else if elem.Name.Local == "tc" {
				textBuilder.WriteString(" | ")
			}
		}
	}

	return textBuilder.String(), nil
}

// splitText splits text into overlapping character chunks.
func splitText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 800
	}
	if overlap <= 0 {
		overlap = 200
	}

	runes := []rune(text)
	if len(runes) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		if end == len(runes) {
			break
		}

		start += chunkSize - overlap
	}

	return chunks
}
