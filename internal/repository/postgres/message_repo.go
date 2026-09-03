package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MessageRepo implements domain.MessageRepository using sqlc-generated chat queries.
type MessageRepo struct {
	db *DB
}

func NewMessageRepo(db *DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// chatMessageToDomain maps a sqlc ChatMessage to the domain entity.
func chatMessageToDomain(m *chatdb.ChatMessage) *domain.Message {
	out := &domain.Message{
		ID:         m.ID,
		SessionID:  m.SessionID,
		SenderType: m.SenderType,
		Content:    m.Content,
		IsLearned:  m.IsLearned,
		CreatedAt:  m.CreatedAt,
	}
	if m.SenderID.Valid {
		out.SenderID = m.SenderID.String
	}
	if m.ClientMsgID.Valid {
		id := uuid.UUID(m.ClientMsgID.Bytes)
		out.ClientMsgID = &id
	}
	return out
}

// unlearnedRowToDomain maps a sqlc GetUnlearnedMessagesRow (no ClientMsgID)
// to the domain entity. ClientMsgID is always nil here.
func unlearnedRowToDomain(r *chatdb.GetUnlearnedMessagesRow) *domain.Message {
	out := &domain.Message{
		ID:         r.ID,
		SessionID:  r.SessionID,
		SenderType: r.SenderType,
		Content:    r.Content,
		IsLearned:  false,
		CreatedAt:  r.CreatedAt,
	}
	if r.SenderID.Valid {
		out.SenderID = r.SenderID.String
	}
	return out
}

// insertMessageBatch is the batch variant of InsertMessage that uses
// ON CONFLICT DO NOTHING so duplicate client_msg_ids are silently skipped.
const insertMessageBatch = `INSERT INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
VALUES ($1, $2, $3, $4, $5, FALSE, NOW())
ON CONFLICT (client_msg_id) WHERE client_msg_id IS NOT NULL DO NOTHING`

// Insert inserts a single message. ON CONFLICT updates content by client_msg_id.
func (r *MessageRepo) Insert(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	clientMsgID := pgtype.UUID{}
	if msg.ClientMsgID != nil {
		clientMsgID = pgtype.UUID{Bytes: *msg.ClientMsgID, Valid: true}
	}

	row, err := r.db.Chat.InsertMessage(ctx, chatdb.InsertMessageParams{
		SessionID:   msg.SessionID,
		SenderType:  msg.SenderType,
		SenderID:    pgtype.Text{String: msg.SenderID, Valid: msg.SenderID != ""},
		Content:     msg.Content,
		ClientMsgID: clientMsgID,
	})
	if err != nil {
		return nil, err
	}
	return chatMessageToDomain(&row), nil
}

// InsertBatch inserts many messages in a single pgx Batch. Duplicates by
// client_msg_id are silently ignored (DO NOTHING).
func (r *MessageRepo) InsertBatch(ctx context.Context, msgs []*domain.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, m := range msgs {
		clientMsgID := pgtype.UUID{}
		if m.ClientMsgID != nil {
			clientMsgID = pgtype.UUID{Bytes: *m.ClientMsgID, Valid: true}
		}
		batch.Queue(
			insertMessageBatch,
			m.SessionID,
			string(m.SenderType),
			pgtype.Text{String: m.SenderID, Valid: m.SenderID != ""},
			m.Content,
			clientMsgID,
		)
	}

	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(msgs); i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// GetHistory returns all messages for a session ordered by created_at ASC.
func (r *MessageRepo) GetHistory(ctx context.Context, sessionID string) ([]*domain.Message, error) {
	rows, err := r.db.Chat.GetSessionHistory(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Message, 0, len(rows))
	for i := range rows {
		out = append(out, chatMessageToDomain(&rows[i]))
	}
	return out, nil
}

// GetUnlearned returns all messages that have not been learned yet.
func (r *MessageRepo) GetUnlearned(ctx context.Context) ([]*domain.Message, error) {
	rows, err := r.db.Chat.GetUnlearnedMessages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Message, 0, len(rows))
	for i := range rows {
		out = append(out, unlearnedRowToDomain(&rows[i]))
	}
	return out, nil
}

// MarkLearned flags messages as learned.
func (r *MessageRepo) MarkLearned(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Chat.MarkMessagesLearned(ctx, ids)
}

// DeleteBySession deletes all messages for the given session.
func (r *MessageRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	return r.db.Chat.DeleteSessionMessages(ctx, sessionID)
}

// DeleteAll deletes every chat message.
func (r *MessageRepo) DeleteAll(ctx context.Context) error {
	return r.db.Chat.DeleteAllMessages(ctx)
}

// ResetLearnedFlags resets all messages' is_learned flag to FALSE.
func (r *MessageRepo) ResetLearnedFlags(ctx context.Context) error {
	return r.db.Chat.ResetLearnedFlags(ctx)
}
