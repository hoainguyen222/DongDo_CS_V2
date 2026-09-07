package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// ============================================================
// Guest Repository
// ============================================================

// GuestRepo persists guest (pre-chat) records via sqlc-generated chat queries.
type GuestRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewGuestRepo constructs a GuestRepo using the shared DB handle.
func NewGuestRepo(db *DB) *GuestRepo {
	return &GuestRepo{
		db:     db,
		logger: logger.With().Str("repo", "guest").Logger(),
	}
}

// Create inserts a new guest record and returns the materialized row.
func (r *GuestRepo) Create(ctx context.Context, guestID uuid.UUID, displayName, phone string) (*domain.Guest, error) {
	g, err := r.db.Chat.CreateGuest(ctx, chatdb.CreateGuestParams{
		GuestID:     guestID,
		DisplayName: displayName,
		Phone:       textFromString(phone),
	})
	if err != nil {
		r.logger.Error().Err(err).Str("guest_id", guestID.String()).Msg("CreateGuest failed")
		return nil, err
	}

	return &domain.Guest{
		ID:          g.ID,
		GuestID:     g.GuestID,
		DisplayName: g.DisplayName,
		Phone:       textToString(g.Phone),
		CreatedAt:   g.CreatedAt,
	}, nil
}

// GetByID returns a guest by guest_id. Returns (nil, nil) when not found.
func (r *GuestRepo) GetByID(ctx context.Context, guestID uuid.UUID) (*domain.Guest, error) {
	g, err := r.db.Chat.GetGuestByID(ctx, guestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error().Err(err).Str("guest_id", guestID.String()).Msg("GetGuestByID failed")
		return nil, err
	}

	return &domain.Guest{
		ID:          g.ID,
		GuestID:     g.GuestID,
		DisplayName: g.DisplayName,
		Phone:       textToString(g.Phone),
		CreatedAt:   g.CreatedAt,
	}, nil
}

// List returns all guests with their latest associated chat case (if any).
func (r *GuestRepo) List(ctx context.Context) ([]*domain.CustomerProfile, error) {
	rows, err := r.db.Chat.ListGuestsWithLastCase(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListGuestsWithLastCase failed")
		return nil, err
	}

	profiles := make([]*domain.CustomerProfile, 0, len(rows))
	for _, row := range rows {
		profiles = append(profiles, &domain.CustomerProfile{
			ID:            row.ID,
			GuestID:       row.GuestID,
			DisplayName:   row.DisplayName,
			Phone:         row.Phone,
			LastSessionID: row.LastSessionID,
			LastMessage:   row.LastMessage,
			LastStatus:    stringFromInterface(row.LastStatus),
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	return profiles, nil
}

// Update updates a guest's display name and phone and synchronizes any active
// chat cases with the new customer name and phone.
func (r *GuestRepo) Update(ctx context.Context, guestID uuid.UUID, displayName, phone string) error {
	if err := r.db.Chat.UpdateGuest(ctx, chatdb.UpdateGuestParams{
		GuestID:     guestID,
		DisplayName: displayName,
		Phone:       textFromString(phone),
	}); err != nil {
		r.logger.Error().Err(err).Str("guest_id", guestID.String()).Msg("UpdateGuest failed")
		return err
	}

	// Best-effort sync of active chat cases; ignore failures to preserve
	// original behavior where the chat_cases update was non-fatal.
	if err := r.db.Chat.SyncActiveCasesForGuest(ctx, chatdb.SyncActiveCasesForGuestParams{
		GuestID:       pgtype.UUID{Bytes: guestID, Valid: true},
		CustomerName:  displayName,
		CustomerPhone: phone,
	}); err != nil {
		r.logger.Warn().Err(err).Str("guest_id", guestID.String()).Msg("sync active cases failed (non-fatal)")
	}

	return nil
}

// Delete removes a guest by guest_id.
func (r *GuestRepo) Delete(ctx context.Context, guestID uuid.UUID) error {
	if err := r.db.Chat.DeleteGuest(ctx, guestID); err != nil {
		r.logger.Error().Err(err).Str("guest_id", guestID.String()).Msg("DeleteGuest failed")
		return err
	}
	return nil
}

// stringFromInterface converts an interface{} (typically a string from a
// sqlc row using an enum cast) into a plain Go string.
func stringFromInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}