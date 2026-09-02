package qdrant

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	qdrantPb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const CollectionName = "dongdo_knowledge"

type Client struct {
	conn       *grpc.ClientConn
	points     qdrantPb.PointsClient
	collection qdrantPb.CollectionsClient
}

func NewClient(ctx context.Context, host string, port int, vectorSize uint64) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant at %s: %w", addr, err)
	}

	c := &Client{
		conn:       conn,
		points:     qdrantPb.NewPointsClient(conn),
		collection: qdrantPb.NewCollectionsClient(conn),
	}

	// Ensure collection exists
	if err := c.ensureCollection(ctx, vectorSize); err != nil {
		log.Printf("⚠️ Qdrant collection check warning: %v", err)
	} else {
		log.Printf("✅ Connected to Qdrant at %s (Collection: %s)", addr, CollectionName)
	}

	return c, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) ensureCollection(ctx context.Context, vectorSize uint64) error {
	if vectorSize == 0 {
		vectorSize = 384 // Default for all-MiniLM-L6-v2
	}

	res, err := c.collection.Get(ctx, &qdrantPb.GetCollectionInfoRequest{
		CollectionName: CollectionName,
	})
	if err == nil && res.Result != nil {
		return nil // Collection already exists
	}

	// Create collection
	_, err = c.collection.Create(ctx, &qdrantPb.CreateCollection{
		CollectionName: CollectionName,
		VectorsConfig: &qdrantPb.VectorsConfig{
			Config: &qdrantPb.VectorsConfig_Params{
				Params: &qdrantPb.VectorParams{
					Size:     vectorSize,
					Distance: qdrantPb.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Qdrant collection %s: %w", CollectionName, err)
	}

	log.Printf("📦 Created Qdrant collection: %s (size: %d, distance: Cosine)", CollectionName, vectorSize)
	return nil
}

// Search queries Qdrant with a vector and returns top matching documents.
func (c *Client) Search(ctx context.Context, queryVector []float32, limit int, scoreThreshold float32) ([]*domain.KnowledgeDocument, error) {
	if limit <= 0 {
		limit = 5
	}

	res, err := c.points.Search(ctx, &qdrantPb.SearchPoints{
		CollectionName: CollectionName,
		Vector:         queryVector,
		Limit:          uint64(limit),
		WithPayload: &qdrantPb.WithPayloadSelector{
			SelectorOptions: &qdrantPb.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		ScoreThreshold: &scoreThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	docs := make([]*domain.KnowledgeDocument, 0, len(res.Result))
	for _, p := range res.Result {
		content := ""
		source := ""
		metaMap := make(map[string]interface{})

		if p.Payload != nil {
			if val, ok := p.Payload["content"]; ok {
				content = val.GetStringValue()
			}
			if val, ok := p.Payload["source"]; ok {
				source = val.GetStringValue()
			}
			for k, v := range p.Payload {
				metaMap[k] = v.GetStringValue()
			}
		}

		docID := ""
		if p.Id != nil {
			if strID, ok := p.Id.PointIdOptions.(*qdrantPb.PointId_Uuid); ok {
				docID = strID.Uuid
			} else if numID, ok := p.Id.PointIdOptions.(*qdrantPb.PointId_Num); ok {
				docID = fmt.Sprintf("%d", numID.Num)
			}
		}

		docs = append(docs, &domain.KnowledgeDocument{
			ID:       docID,
			Content:  content,
			Score:    p.Score,
			Source:   source,
			Metadata: metaMap,
		})
	}

	return docs, nil
}

// Upsert adds or updates points in Qdrant.
func (c *Client) Upsert(ctx context.Context, docs []*domain.KnowledgeDocument, vectors [][]float32) error {
	if len(docs) != len(vectors) {
		return fmt.Errorf("length mismatch: %d docs vs %d vectors", len(docs), len(vectors))
	}

	points := make([]*qdrantPb.PointStruct, len(docs))
	for i, doc := range docs {
		payload := map[string]*qdrantPb.Value{
			"content": {Kind: &qdrantPb.Value_StringValue{StringValue: doc.Content}},
			"source":  {Kind: &qdrantPb.Value_StringValue{StringValue: doc.Source}},
		}
		for k, v := range doc.Metadata {
			payload[k] = &qdrantPb.Value{Kind: &qdrantPb.Value_StringValue{StringValue: fmt.Sprintf("%v", v)}}
		}

		pointUUID := doc.ID
		if _, err := uuid.Parse(pointUUID); err != nil {
			pointUUID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(doc.ID)).String()
		}

		points[i] = &qdrantPb.PointStruct{
			Id: &qdrantPb.PointId{
				PointIdOptions: &qdrantPb.PointId_Uuid{
					Uuid: pointUUID,
				},
			},
			Vectors: &qdrantPb.Vectors{
				VectorsOptions: &qdrantPb.Vectors_Vector{
					Vector: &qdrantPb.Vector{
						Data: vectors[i],
					},
				},
			},
			Payload: payload,
		}
	}

	_, err := c.points.Upsert(ctx, &qdrantPb.UpsertPoints{
		CollectionName: CollectionName,
		Points:         points,
	})
	return err
}

// DeleteBySource deletes points matching a source filter.
func (c *Client) DeleteBySource(ctx context.Context, source string) (int, error) {
	res, err := c.points.Delete(ctx, &qdrantPb.DeletePoints{
		CollectionName: CollectionName,
		Points: &qdrantPb.PointsSelector{
			PointsSelectorOneOf: &qdrantPb.PointsSelector_Filter{
				Filter: &qdrantPb.Filter{
					Must: []*qdrantPb.Condition{
						{
							ConditionOneOf: &qdrantPb.Condition_Field{
								Field: &qdrantPb.FieldCondition{
									Key: "source",
									Match: &qdrantPb.Match{
										MatchValue: &qdrantPb.Match_Keyword{
											Keyword: source,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return 0, err
	}
	_ = res
	return 1, nil
}

// Count returns the number of points in the collection.
func (c *Client) Count(ctx context.Context) (int64, error) {
	res, err := c.points.Count(ctx, &qdrantPb.CountPoints{
		CollectionName: CollectionName,
	})
	if err != nil {
		return 0, err
	}
	return int64(res.Result.Count), nil
}
