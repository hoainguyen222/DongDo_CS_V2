package main

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraEmbedding "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/embedding"
	infraQdrant "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/qdrant"
)

func main() {
	log.Println("============================================================")
	log.Println("🚀 ĐÔNG ĐÔ CS V2 - Document Ingestion Pipeline (Qdrant)")
	log.Println("============================================================")

	cfg := config.Load()
	ctx := context.Background()

	// 1. Connect to Qdrant
	qdrantClient, err := infraQdrant.NewClient(ctx, cfg.QdrantHost, cfg.QdrantPort, 384)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Qdrant: %v", err)
	}
	defer qdrantClient.Close()

	// 2. Initialize Embedder
	embedder := infraEmbedding.NewEmbedder(cfg.EmbeddingModel)

	// 3. Find .docx files
	docsDir := cfg.DocumentsDir
	files, err := filepath.Glob(filepath.Join(docsDir, "*.docx"))
	if err != nil || len(files) == 0 {
		log.Printf("⚠️ No .docx files found in directory: %s", docsDir)
		return
	}

	log.Printf("📂 Found %d document(s) in %s\n", len(files), docsDir)

	totalChunks := 0

	for _, fpath := range files {
		fname := filepath.Base(fpath)
		log.Printf("📄 Reading: %s", fname)

		text, err := extractTextFromDocx(fpath)
		if err != nil || strings.TrimSpace(text) == "" {
			log.Printf("   ⚠️ Failed to read or empty: %v", err)
			continue
		}

		log.Printf("   ✅ Extracted %d characters", len(text))

		chunks := splitText(text, cfg.ChunkSize, cfg.ChunkOverlap)
		log.Printf("   ✂️ Created %d chunks", len(chunks))

		// Embed and Upsert
		docs := make([]*domain.KnowledgeDocument, len(chunks))
		for i, chunk := range chunks {
			docs[i] = &domain.KnowledgeDocument{
				ID:      uuid.New().String(),
				Content: chunk,
				Source:  fname,
				Metadata: map[string]interface{}{
					"source":      fname,
					"chunk_id":    fmt.Sprintf("%d", i),
					"ingested_at": time.Now().Format(time.RFC3339),
					"type":        "knowledge_base",
				},
			}
		}

		vectors, err := embedder.EmbedBatch(ctx, chunks)
		if err != nil {
			log.Printf("   ❌ Embedding failed: %v", err)
			continue
		}

		err = qdrantClient.Upsert(ctx, docs, vectors)
		if err != nil {
			log.Printf("   ❌ Qdrant upsert failed: %v", err)
			continue
		}

		log.Printf("   📦 Ingested %d chunks into Qdrant", len(chunks))
		totalChunks += len(chunks)
	}

	log.Println("============================================================")
	log.Printf("✅ INGEST COMPLETE! Total %d chunks stored in Qdrant.", totalChunks)
	log.Println("============================================================")
}

// extractTextFromDocx extracts all paragraphs from a .docx file (word/document.xml in zip).
func extractTextFromDocx(filepath string) (string, error) {
	r, err := zip.OpenReader(filepath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var docXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", err
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
