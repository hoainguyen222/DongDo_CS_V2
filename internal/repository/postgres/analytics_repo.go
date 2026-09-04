package postgres

import (
	"context"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// AnalyticsRepo implements domain.AnalyticsRepository using sqlc-generated analytics queries.
type AnalyticsRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewAnalyticsRepo constructs an AnalyticsRepo using the shared DB handle.
func NewAnalyticsRepo(db *DB) *AnalyticsRepo {
	return &AnalyticsRepo{
		db:     db,
		logger: logger.With().Str("repo", "analytics").Logger(),
	}
}

// GetStats returns aggregate analytics statistics. The AIServiceRate is computed
// from AIActiveCases / TotalCases and rounded to one decimal place.
func (r *AnalyticsRepo) GetStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	row, err := r.db.Analytics.GetAnalyticsStats(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("GetAnalyticsStats failed")
		return nil, err
	}

	totalCases := int(row.TotalCases)
	aiServiceRate := 0.0
	if totalCases > 0 {
		aiServiceRate = float64(int((float64(row.AiActiveCases)/float64(totalCases))*1000)) / 10.0
	}

	return &domain.AnalyticsStats{
		TotalCases:        totalCases,
		TotalSessions:     int(row.TotalSessions),
		AIActiveCases:     int(row.AiActiveCases),
		NeedsHumanCases:   int(row.NeedsHumanCases),
		ActiveHumanCases:  int(row.ActiveHumanCases),
		ResolvedCases:     int(row.ResolvedCases),
		AIServiceRate:     aiServiceRate,
		TotalLearnedQA:    int(row.TotalLearnedQa),
		PendingLearnCount: int(row.PendingLearnCount),
	}, nil
}