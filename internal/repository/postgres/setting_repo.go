package postgres

import (
	"context"
	"errors"

	settingsdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/settings"
	"github.com/jackc/pgx/v5"
)

// SettingRepo implements domain.SettingRepository using sqlc-generated settings queries.
type SettingRepo struct {
	db *DB
}

// NewSettingRepo constructs a SettingRepo using the shared DB handle.
func NewSettingRepo(db *DB) *SettingRepo {
	return &SettingRepo{db: db}
}

// Get returns the value for a settings key, falling back to the provided default
// when the key does not exist. Any error other than ErrNoRows is returned with
// the fallback value.
func (r *SettingRepo) Get(ctx context.Context, key, fallback string) (string, error) {
	val, err := r.db.Settings.GetSetting(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fallback, nil
		}
		return fallback, err
	}
	return val, nil
}

// Set upserts a settings key-value pair.
func (r *SettingRepo) Set(ctx context.Context, key, value string) error {
	return r.db.Settings.UpsertSetting(ctx, settingsdb.UpsertSettingParams{
		SettingKey:   key,
		SettingValue: value,
	})
}
