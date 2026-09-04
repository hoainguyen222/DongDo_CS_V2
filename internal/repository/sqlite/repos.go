package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// User & Session & Guest Repositories
// ============================================================

type UserRepo struct{ db *DB }
func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, username, passwordHash, salt, fullName string, role domain.UserRole) (*domain.User, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)
	`, username, passwordHash, salt, fullName, string(role), now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	t, _ := time.Parse(time.RFC3339, now)
	return &domain.User{ID: id, Username: username, FullName: fullName, Role: role, IsActive: true, CreatedAt: t}, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `
		SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
		FROM users WHERE LOWER(username) = LOWER(?)
	`, username)

	var u domain.User
	var roleStr, createdAtStr string
	var isActiveInt int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Salt, &u.FullName, &roleStr, &isActiveInt, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.Role = domain.UserRole(roleStr)
	u.IsActive = isActiveInt == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &u, nil
}

func (r *UserRepo) List(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `SELECT id, username, full_name, role, is_active, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		var roleStr, createdAtStr string
		var isActiveInt int
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &roleStr, &isActiveInt, &createdAtStr); err != nil {
			return nil, err
		}
		u.Role = domain.UserRole(roleStr)
		u.IsActive = isActiveInt == 1
		u.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		users = append(users, &u)
	}
	return users, nil
}

func (r *UserRepo) Delete(ctx context.Context, username string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username)
	return err
}

func (r *UserRepo) UpdatePassword(ctx context.Context, username, passwordHash, salt string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE users SET password_hash = ?, salt = ? WHERE username = ?`, passwordHash, salt, username)
	return err
}

func (r *UserRepo) UpdateUser(ctx context.Context, username, fullName string, role domain.UserRole, isActive bool, passwordHash, salt string) (*domain.User, error) {
	isActiveInt := 0
	if isActive {
		isActiveInt = 1
	}

	if passwordHash != "" {
		_, err := r.db.SQLDB.ExecContext(ctx, `
			UPDATE users SET full_name = ?, role = ?, is_active = ?, password_hash = ?, salt = ?
			WHERE LOWER(username) = LOWER(?)
		`, fullName, string(role), isActiveInt, passwordHash, salt, username)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := r.db.SQLDB.ExecContext(ctx, `
			UPDATE users SET full_name = ?, role = ?, is_active = ?
			WHERE LOWER(username) = LOWER(?)
		`, fullName, string(role), isActiveInt, username)
		if err != nil {
			return nil, err
		}
	}
	return r.GetByUsername(ctx, username)
}

// Session Repository
type SessionRepo struct{ db *DB }
func NewSessionRepo(db *DB) *SessionRepo { return &SessionRepo{db: db} }

func (r *SessionRepo) Create(ctx context.Context, token, username string, expiresAt time.Time) (*domain.Session, error) {
	now := time.Now().Format(time.RFC3339)
	expStr := expiresAt.Format(time.RFC3339)
	res, err := r.db.SQLDB.ExecContext(ctx, `INSERT INTO sessions (token, username, created_at, expires_at) VALUES (?, ?, ?, ?)`, token, username, now, expStr)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.Session{ID: id, Token: token, Username: username, CreatedAt: time.Now(), ExpiresAt: expiresAt}, nil
}

func (r *SessionRepo) Verify(ctx context.Context, token string) (*domain.SessionUser, error) {
	nowStr := time.Now().Format(time.RFC3339)
	row := r.db.SQLDB.QueryRowContext(ctx, `
		SELECT s.username, u.full_name, u.role
		FROM sessions s
		JOIN users u ON s.username = u.username
		WHERE s.token = ? AND s.expires_at > ? AND u.is_active = 1
	`, token, nowStr)

	var su domain.SessionUser
	var roleStr string
	err := row.Scan(&su.Username, &su.FullName, &roleStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	su.Role = domain.UserRole(roleStr)
	su.Token = token
	return &su, nil
}

func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}
func (r *SessionRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().Format(time.RFC3339))
	return err
}
func (r *SessionRepo) DeleteByUsername(ctx context.Context, username string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM sessions WHERE username = ?`, username)
	return err
}

// Guest Repository
type GuestRepo struct{ db *DB }
func NewGuestRepo(db *DB) *GuestRepo { return &GuestRepo{db: db} }

func (r *GuestRepo) Create(ctx context.Context, guestID uuid.UUID, displayName, phone string) (*domain.Guest, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.SQLDB.ExecContext(ctx, `INSERT INTO guests (guest_id, display_name, phone, created_at) VALUES (?, ?, ?, ?)`, guestID.String(), displayName, phone, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.Guest{ID: id, GuestID: guestID, DisplayName: displayName, Phone: phone, CreatedAt: time.Now()}, nil
}

func (r *GuestRepo) GetByID(ctx context.Context, guestID uuid.UUID) (*domain.Guest, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `SELECT id, guest_id, display_name, phone, created_at FROM guests WHERE guest_id = ?`, guestID.String())
	var g domain.Guest
	var gIDStr, cAtStr string
	err := row.Scan(&g.ID, &gIDStr, &g.DisplayName, &g.Phone, &cAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	g.GuestID, _ = uuid.Parse(gIDStr)
	g.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
	return &g, nil
}

func (r *GuestRepo) List(ctx context.Context) ([]*domain.CustomerProfile, error) {
	query := `
		SELECT 
			g.id,
			g.guest_id,
			g.display_name,
			COALESCE(g.phone, '') as phone,
			COALESCE(c.session_id, '') as last_session_id,
			COALESCE(c.last_message, '') as last_message,
			COALESCE(c.status, '') as last_status,
			g.created_at,
			COALESCE(c.updated_at, g.created_at) as updated_at
		FROM guests g
		LEFT JOIN chat_cases c ON c.guest_id = g.guest_id
		ORDER BY g.created_at DESC
	`
	rows, err := r.db.SQLDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*domain.CustomerProfile
	for rows.Next() {
		var p domain.CustomerProfile
		var cAtStr, uAtStr string
		if err := rows.Scan(
			&p.ID,
			&p.GuestID,
			&p.DisplayName,
			&p.Phone,
			&p.LastSessionID,
			&p.LastMessage,
			&p.LastStatus,
			&cAtStr,
			&uAtStr,
		); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, uAtStr)
		profiles = append(profiles, &p)
	}
	return profiles, nil
}

func (r *GuestRepo) Update(ctx context.Context, guestID uuid.UUID, displayName, phone string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE guests SET display_name = ?, phone = ? WHERE guest_id = ?`, displayName, phone, guestID.String())
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	_, _ = r.db.SQLDB.ExecContext(ctx, `UPDATE chat_cases SET customer_name = ?, customer_phone = ?, updated_at = ? WHERE guest_id = ?`, displayName, phone, now, guestID.String())
	return nil
}

func (r *GuestRepo) Delete(ctx context.Context, guestID uuid.UUID) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM guests WHERE guest_id = ?`, guestID.String())
	return err
}

// ============================================================
// Message Repository
// ============================================================

type MessageRepo struct{ db *DB }
func NewMessageRepo(db *DB) *MessageRepo { return &MessageRepo{db: db} }

func (r *MessageRepo) Insert(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	now := time.Now().Format(time.RFC3339)
	var cMsgIDStr *string
	if msg.ClientMsgID != nil {
		s := msg.ClientMsgID.String()
		cMsgIDStr = &s
	}

	res, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`, msg.SessionID, string(msg.SenderType), msg.SenderID, msg.Content, cMsgIDStr, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	msg.ID = id
	msg.CreatedAt = time.Now()
	return msg, nil
}

func (r *MessageRepo) InsertBatch(ctx context.Context, msgs []*domain.Message) error {
	tx, err := r.db.SQLDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)
	for _, m := range msgs {
		var cMsgIDStr *string
		if m.ClientMsgID != nil {
			s := m.ClientMsgID.String()
			cMsgIDStr = &s
		}
		_, err := stmt.ExecContext(ctx, m.SessionID, string(m.SenderType), m.SenderID, m.Content, cMsgIDStr, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *MessageRepo) GetHistory(ctx context.Context, sessionID string) ([]*domain.Message, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `
		SELECT id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at
		FROM chat_messages WHERE session_id = ? ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		var m domain.Message
		var sType, cAtStr string
		var cMsgIDStr sql.NullString
		var isLearnedInt int
		if err := rows.Scan(&m.ID, &m.SessionID, &sType, &m.SenderID, &m.Content, &cMsgIDStr, &isLearnedInt, &cAtStr); err != nil {
			return nil, err
		}
		m.SenderType = domain.SenderType(sType)
		m.IsLearned = isLearnedInt == 1
		m.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		if cMsgIDStr.Valid {
			u, _ := uuid.Parse(cMsgIDStr.String)
			m.ClientMsgID = &u
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *MessageRepo) GetUnlearned(ctx context.Context) ([]*domain.Message, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at FROM chat_messages WHERE is_learned = 0 ORDER BY session_id, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		var m domain.Message
		var sType, cAtStr string
		var cMsgIDStr sql.NullString
		var isLearnedInt int
		if err := rows.Scan(&m.ID, &m.SessionID, &sType, &m.SenderID, &m.Content, &cMsgIDStr, &isLearnedInt, &cAtStr); err != nil {
			return nil, err
		}
		m.SenderType = domain.SenderType(sType)
		m.IsLearned = isLearnedInt == 1
		m.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *MessageRepo) MarkLearned(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "UPDATE chat_messages SET is_learned = 1 WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := r.db.SQLDB.ExecContext(ctx, query, args...)
	return err
}

func (r *MessageRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM chat_messages WHERE session_id = ?`, sessionID)
	return err
}

func (r *MessageRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM chat_messages`)
	return err
}

func (r *MessageRepo) ResetLearnedFlags(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE chat_messages SET is_learned = 0`)
	return err
}

// ============================================================
// Case Repository
// ============================================================

type CaseRepo struct{ db *DB }

func NewCaseRepo(db *DB) *CaseRepo { return &CaseRepo{db: db} }

func (r *CaseRepo) Upsert(ctx context.Context, sessionID string, guestID *uuid.UUID, customerName, customerPhone string, status domain.CaseStatus, assignedCS, lastMessage string) (*domain.ChatCase, error) {
	now := time.Now().Format(time.RFC3339)
	var gIDStr *string
	if guestID != nil {
		s := guestID.String()
		gIDStr = &s
	}

	var existingStatus, existingCS string
	err := r.db.SQLDB.QueryRowContext(ctx, `SELECT status, assigned_cs FROM chat_cases WHERE session_id = ?`, sessionID).Scan(&existingStatus, &existingCS)
	if err == nil {
		// Update
		newStatus := string(status)
		if existingStatus == "HUMAN_CS_ACTIVE" && status == domain.StatusNeedsHumanCS {
			newStatus = "HUMAN_CS_ACTIVE"
		}
		newCS := assignedCS
		if newCS == "" {
			newCS = existingCS
		}
		_, err = r.db.SQLDB.ExecContext(ctx, `
			UPDATE chat_cases SET status = ?, assigned_cs = ?, last_message = COALESCE(NULLIF(?, ''), last_message), updated_at = ?
			WHERE session_id = ?
		`, newStatus, newCS, lastMessage, now, sessionID)
	} else {
		// Insert
		custName := customerName
		if custName == "" {
			custName = "Khách hàng"
		}
		_, err = r.db.SQLDB.ExecContext(ctx, `
			INSERT INTO chat_cases (session_id, guest_id, customer_name, status, assigned_cs, last_message, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, sessionID, gIDStr, custName, string(status), assignedCS, lastMessage, now, now)
	}

	return r.Get(ctx, sessionID)
}

func (r *CaseRepo) List(ctx context.Context, statusFilter domain.CaseStatus) ([]*domain.ChatCase, error) {
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at FROM chat_cases WHERE status = ? ORDER BY updated_at DESC`, string(statusFilter))
	} else {
		rows, err = r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at FROM chat_cases ORDER BY updated_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []*domain.ChatCase
	for rows.Next() {
		var c domain.ChatCase
		var statusStr, cAtStr, uAtStr string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.CustomerName, &statusStr, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Status = domain.CaseStatus(statusStr)
		c.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, uAtStr)
		cases = append(cases, &c)
	}
	return cases, nil
}

func (r *CaseRepo) Get(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `SELECT id, session_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at FROM chat_cases WHERE session_id = ?`, sessionID)
	var c domain.ChatCase
	var statusStr, cAtStr, uAtStr string
	err := row.Scan(&c.ID, &c.SessionID, &c.CustomerName, &statusStr, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.Status = domain.CaseStatus(statusStr)
	c.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, uAtStr)
	return &c, nil
}

func (r *CaseRepo) Assign(ctx context.Context, sessionID, csUsername string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE chat_cases SET status = 'HUMAN_CS_ACTIVE', assigned_cs = ?, updated_at = ? WHERE session_id = ?`, csUsername, now, sessionID)
	return err
}

func (r *CaseRepo) Resolve(ctx context.Context, sessionID, csUsername, resolutionNote string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE chat_cases SET status = 'RESOLVED', assigned_cs = ?, resolution_note = ?, updated_at = ? WHERE session_id = ?`, csUsername, resolutionNote, now, sessionID)
	return err
}

func (r *CaseRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM chat_cases WHERE session_id = ?`, sessionID)
	return err
}

func (r *CaseRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM chat_cases`)
	return err
}

// ============================================================
// Learning, Settings, Voice & Analytics Repositories
// ============================================================

type LearningRepo struct{ db *DB }
func NewLearningRepo(db *DB) *LearningRepo { return &LearningRepo{db: db} }

func (r *LearningRepo) Add(ctx context.Context, sessionID, question, answer string, status domain.LearnStatus, createdBy string) (*domain.LearningItem, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.SQLDB.ExecContext(ctx, `INSERT INTO learning_queue (session_id, question, answer, status, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`, sessionID, question, answer, string(status), createdBy, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.LearningItem{ID: id, SessionID: sessionID, Question: question, Answer: answer, Status: status, CreatedBy: createdBy, CreatedAt: time.Now()}, nil
}

func (r *LearningRepo) ListByStatus(ctx context.Context, status domain.LearnStatus) ([]*domain.LearningItem, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at FROM learning_queue WHERE status = ? ORDER BY id DESC`, string(status))
	} else {
		rows, err = r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at FROM learning_queue ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.LearningItem
	for rows.Next() {
		var item domain.LearningItem
		var statusStr, cAtStr string
		var approvedByStr, appAtStr sql.NullString
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Question, &item.Answer, &statusStr, &item.CreatedBy, &approvedByStr, &cAtStr, &appAtStr); err != nil {
			return nil, err
		}
		item.Status = domain.LearnStatus(statusStr)
		item.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		if approvedByStr.Valid {
			item.ApprovedBy = approvedByStr.String
		}
		items = append(items, &item)
	}
	return items, nil
}

func (r *LearningRepo) Get(ctx context.Context, id int64) (*domain.LearningItem, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at FROM learning_queue WHERE id = ?`, id)
	var item domain.LearningItem
	var statusStr, cAtStr string
	var approvedByStr, appAtStr sql.NullString
	err := row.Scan(&item.ID, &item.SessionID, &item.Question, &item.Answer, &statusStr, &item.CreatedBy, &approvedByStr, &cAtStr, &appAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.Status = domain.LearnStatus(statusStr)
	item.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
	return &item, nil
}

func (r *LearningRepo) UpdateContent(ctx context.Context, id int64, question, answer string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE learning_queue SET question = ?, answer = ? WHERE id = ?`, question, answer, id)
	return err
}

func (r *LearningRepo) MarkStatus(ctx context.Context, id int64, status domain.LearnStatus, approvedBy string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE learning_queue SET status = ?, approved_by = ?, approved_at = ? WHERE id = ?`, string(status), approvedBy, now, id)
	return err
}

func (r *LearningRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM learning_queue WHERE session_id = ? AND status = 'PENDING'`, sessionID)
	return err
}

func (r *LearningRepo) ClearAll(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM learning_queue`)
	return err
}

// Setting Repository
type SettingRepo struct{ db *DB }
func NewSettingRepo(db *DB) *SettingRepo { return &SettingRepo{db: db} }

func (r *SettingRepo) Get(ctx context.Context, key, fallback string) (string, error) {
	var val string
	err := r.db.SQLDB.QueryRowContext(ctx, `SELECT setting_value FROM system_settings WHERE setting_key = ?`, key).Scan(&val)
	if err != nil {
		return fallback, nil
	}
	return val, nil
}

func (r *SettingRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `INSERT OR REPLACE INTO system_settings (setting_key, setting_value) VALUES (?, ?)`, key, value)
	return err
}

// VoiceCall Repository
type VoiceCallRepo struct{ db *DB }
func NewVoiceCallRepo(db *DB) *VoiceCallRepo { return &VoiceCallRepo{db: db} }

func (r *VoiceCallRepo) Create(ctx context.Context, call *domain.VoiceCall) (*domain.VoiceCall, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT INTO voice_calls (session_id, caller_type, caller_id, callee_type, callee_id, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'RINGING', ?)
	`, call.SessionID, string(call.CallerType), call.CallerID, string(call.CalleeType), call.CalleeID, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	call.ID = id
	call.CreatedAt = time.Now()
	return call, nil
}

func (r *VoiceCallRepo) UpdateStatus(ctx context.Context, id int64, status domain.CallStatus) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE voice_calls SET status = ?, ended_at = ? WHERE id = ?`, string(status), now, id)
	return err
}

func (r *VoiceCallRepo) End(ctx context.Context, id int64, durationSeconds int, recordingURL string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE voice_calls SET status = 'ENDED', duration_seconds = ?, recording_url = ?, ended_at = ? WHERE id = ?`, durationSeconds, recordingURL, now, id)
	return err
}

func (r *VoiceCallRepo) SetTranscript(ctx context.Context, id int64, transcript string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE voice_calls SET transcript = ? WHERE id = ?`, transcript, id)
	return err
}

func (r *VoiceCallRepo) ListAll(ctx context.Context) ([]*domain.VoiceCall, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at FROM voice_calls ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*domain.VoiceCall
	for rows.Next() {
		var vc domain.VoiceCall
		var cType, ceType, sType, cAtStr string
		var endAtStr sql.NullString
		if err := rows.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &cAtStr, &endAtStr); err != nil {
			return nil, err
		}
		vc.CallerType = domain.CallerType(cType)
		vc.CalleeType = domain.CallerType(ceType)
		vc.Status = domain.CallStatus(sType)
		vc.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		calls = append(calls, &vc)
	}
	return calls, nil
}

func (r *VoiceCallRepo) GetBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at FROM voice_calls WHERE session_id = ? ORDER BY id DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*domain.VoiceCall
	for rows.Next() {
		var vc domain.VoiceCall
		var cType, ceType, sType, cAtStr string
		var endAtStr sql.NullString
		if err := rows.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &cAtStr, &endAtStr); err != nil {
			return nil, err
		}
		vc.CallerType = domain.CallerType(cType)
		vc.CalleeType = domain.CallerType(ceType)
		vc.Status = domain.CallStatus(sType)
		vc.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
		calls = append(calls, &vc)
	}
	return calls, nil
}

func (r *VoiceCallRepo) GetByID(ctx context.Context, id int64) (*domain.VoiceCall, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at FROM voice_calls WHERE id = ?`, id)
	var vc domain.VoiceCall
	var cType, ceType, sType, cAtStr string
	var endAtStr sql.NullString
	err := row.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &cAtStr, &endAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	vc.CallerType = domain.CallerType(cType)
	vc.CalleeType = domain.CallerType(ceType)
	vc.Status = domain.CallStatus(sType)
	vc.CreatedAt, _ = time.Parse(time.RFC3339, cAtStr)
	return &vc, nil
}

func (r *VoiceCallRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `DELETE FROM voice_calls WHERE id = ?`, id)
	return err
}

// Analytics Repository
type AnalyticsRepo struct{ db *DB }
func NewAnalyticsRepo(db *DB) *AnalyticsRepo { return &AnalyticsRepo{db: db} }

func (r *AnalyticsRepo) GetStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	var s domain.AnalyticsStats
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_cases`).Scan(&s.TotalCases)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT session_id) FROM chat_messages`).Scan(&s.TotalSessions)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_cases WHERE status = 'AI_ACTIVE'`).Scan(&s.AIActiveCases)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS'`).Scan(&s.NeedsHumanCases)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_cases WHERE status = 'HUMAN_CS_ACTIVE'`).Scan(&s.ActiveHumanCases)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_cases WHERE status = 'RESOLVED'`).Scan(&s.ResolvedCases)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM learning_queue WHERE status = 'APPROVED'`).Scan(&s.TotalLearnedQA)
	_ = r.db.SQLDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM learning_queue WHERE status = 'PENDING'`).Scan(&s.PendingLearnCount)

	if s.TotalCases > 0 {
		s.AIServiceRate = float64(int((float64(s.AIActiveCases)/float64(s.TotalCases))*1000)) / 10.0
	}
	return &s, nil
}