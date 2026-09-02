package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type MessageRepo struct {
	db *DB
}

func NewMessageRepo(db *DB) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) Insert(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
		VALUES ($1, $2::sender_type, $3, $4, $5, FALSE, NOW())
		ON CONFLICT (client_msg_id) WHERE client_msg_id IS NOT NULL DO UPDATE SET content = EXCLUDED.content
		RETURNING id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at
	`, msg.SessionID, string(msg.SenderType), msg.SenderID, msg.Content, msg.ClientMsgID)

	var m domain.Message
	var sType string
	err := row.Scan(&m.ID, &m.SessionID, &sType, &m.SenderID, &m.Content, &m.ClientMsgID, &m.IsLearned, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.SenderType = domain.SenderType(sType)
	return &m, nil
}

// InsertBatch performs an optimized batch insert of messages using pgx.Batch.
func (r *MessageRepo) InsertBatch(ctx context.Context, msgs []*domain.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, m := range msgs {
		batch.Queue(`
			INSERT INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
			VALUES ($1, $2::sender_type, $3, $4, $5, FALSE, NOW())
			ON CONFLICT (client_msg_id) WHERE client_msg_id IS NOT NULL DO NOTHING
		`, m.SessionID, string(m.SenderType), m.SenderID, m.Content, m.ClientMsgID)
	}

	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(msgs); i++ {
		_, err := br.Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *MessageRepo) GetHistory(ctx context.Context, sessionID string) ([]*domain.Message, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at
		FROM chat_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		var m domain.Message
		var sType string
		if err := rows.Scan(&m.ID, &m.SessionID, &sType, &m.SenderID, &m.Content, &m.ClientMsgID, &m.IsLearned, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SenderType = domain.SenderType(sType)
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *MessageRepo) GetUnlearned(ctx context.Context) ([]*domain.Message, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at
		FROM chat_messages
		WHERE is_learned = FALSE
		ORDER BY session_id, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		var m domain.Message
		var sType string
		if err := rows.Scan(&m.ID, &m.SessionID, &sType, &m.SenderID, &m.Content, &m.ClientMsgID, &m.IsLearned, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SenderType = domain.SenderType(sType)
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *MessageRepo) MarkLearned(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_messages SET is_learned = TRUE WHERE id = ANY($1)
	`, ids)
	return err
}

func (r *MessageRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM chat_messages WHERE session_id = $1`, sessionID)
	return err
}

func (r *MessageRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM chat_messages`)
	return err
}

func (r *MessageRepo) ResetLearnedFlags(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE chat_messages SET is_learned = FALSE`)
	return err
}
