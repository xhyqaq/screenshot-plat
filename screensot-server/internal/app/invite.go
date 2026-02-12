package app

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const inviteCodeLength = 12

type InviteStore struct {
	db *sql.DB
}

type InviteRecord struct {
	ID            int64  `json:"id"`
	InviteCode    string `json:"invite_code"`
	CodeHash      string `json:"code_hash"`
	ExpAt         int64  `json:"exp_at"`
	CreatedAt     int64  `json:"created_at"`
	Note          string `json:"note"`
	Revoked       bool   `json:"revoked"`
	BoundDeviceID string `json:"bound_device_id"`
	BoundAt       int64  `json:"bound_at"`
	LastSeenAt    int64  `json:"last_seen_at"`
}

func newInviteStore(path string) (*InviteStore, error) {
	if strings.TrimSpace(path) == "" {
		path = "data/invites.db"
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir failed: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}
	store := &InviteStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *InviteStore) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS invites (
			id INTEGER PRIMARY KEY,
			code_hash TEXT UNIQUE NOT NULL,
			code_plain TEXT,
			exp_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			note TEXT,
			revoked INTEGER NOT NULL DEFAULT 0,
			bound_device_id TEXT,
			bound_at INTEGER,
			last_seen_at INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_invites_code_hash ON invites(code_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_invites_exp_at ON invites(exp_at);`,
		`CREATE INDEX IF NOT EXISTS idx_invites_bound_device_id ON invites(bound_device_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema failed: %w", err)
		}
	}
	if err := s.ensureColumn("code_plain TEXT"); err != nil {
		return err
	}
	return nil
}

func (s *InviteStore) ensureColumn(definition string) error {
	if strings.TrimSpace(definition) == "" {
		return nil
	}
	_, err := s.db.Exec("ALTER TABLE invites ADD COLUMN " + definition)
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists") {
		return nil
	}
	return fmt.Errorf("alter invites failed: %w", err)
}

func (s *InviteStore) CreateInvite(ttlSeconds int64, note string) (string, int64, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 24 * 60 * 60
	}
	now := time.Now().Unix()
	expAt := now + ttlSeconds
	for i := 0; i < 5; i++ {
		code, err := generateInviteCode(inviteCodeLength)
		if err != nil {
			return "", 0, err
		}
		codeHash := hashInviteCode(code)
		_, err = s.db.Exec(
			"INSERT INTO invites (code_hash, code_plain, exp_at, created_at, note) VALUES (?, ?, ?, ?, ?)",
			codeHash, code, expAt, now, note,
		)
		if err == nil {
			return code, expAt, nil
		}
		if !isUniqueConstraintError(err) {
			return "", 0, fmt.Errorf("insert invite failed: %w", err)
		}
	}
	return "", 0, errors.New("failed to generate unique invite code")
}

func (s *InviteStore) ListInvites() ([]InviteRecord, error) {
	rows, err := s.db.Query(`SELECT id, code_plain, code_hash, exp_at, created_at, note, revoked, bound_device_id, bound_at, last_seen_at
		FROM invites ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list invites failed: %w", err)
	}
	defer rows.Close()
	var out []InviteRecord
	for rows.Next() {
		var rec InviteRecord
		var revoked int
		var boundDevice sql.NullString
		var boundAt sql.NullInt64
		var lastSeen sql.NullInt64
		var codePlain sql.NullString
		if err := rows.Scan(&rec.ID, &codePlain, &rec.CodeHash, &rec.ExpAt, &rec.CreatedAt, &rec.Note, &revoked, &boundDevice, &boundAt, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan invite failed: %w", err)
		}
		if codePlain.Valid {
			rec.InviteCode = codePlain.String
		}
		rec.Revoked = revoked != 0
		if boundDevice.Valid {
			rec.BoundDeviceID = boundDevice.String
		}
		if boundAt.Valid {
			rec.BoundAt = boundAt.Int64
		}
		if lastSeen.Valid {
			rec.LastSeenAt = lastSeen.Int64
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list invites failed: %w", err)
	}
	return out, nil
}

func (s *InviteStore) RevokeInvite(inviteCode string) error {
	codeHash := hashInviteCode(strings.TrimSpace(inviteCode))
	if codeHash == "" {
		return errors.New("invite_code required")
	}
	res, err := s.db.Exec("UPDATE invites SET revoked = 1 WHERE code_hash = ?", codeHash)
	if err != nil {
		return fmt.Errorf("revoke invite failed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("invite not found")
	}
	return nil
}

func (s *InviteStore) ResetBinding(inviteCode string) error {
	codeHash := hashInviteCode(strings.TrimSpace(inviteCode))
	if codeHash == "" {
		return errors.New("invite_code required")
	}
	res, err := s.db.Exec(`UPDATE invites
		SET bound_device_id = NULL, bound_at = NULL, last_seen_at = NULL
		WHERE code_hash = ?`, codeHash)
	if err != nil {
		return fmt.Errorf("reset binding failed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("invite not found")
	}
	return nil
}

func (s *InviteStore) Authenticate(inviteCode, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device_id required")
	}
	now := time.Now().Unix()
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode != "" {
		return s.bindOrValidate(inviteCode, deviceID, now)
	}
	return s.validateByDevice(deviceID, now)
}

func (s *InviteStore) ValidateInvite(inviteCode string) error {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return errors.New("invite_code required")
	}
	codeHash := hashInviteCode(inviteCode)
	var expAt int64
	var revoked int
	if err := s.db.QueryRow("SELECT exp_at, revoked FROM invites WHERE code_hash = ?", codeHash).Scan(&expAt, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invite not found")
		}
		return fmt.Errorf("query invite failed: %w", err)
	}
	if revoked != 0 {
		return errors.New("invite revoked")
	}
	if expAt <= time.Now().Unix() {
		return errors.New("invite expired")
	}
	return nil
}

func (s *InviteStore) bindOrValidate(inviteCode, deviceID string, now int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	codeHash := hashInviteCode(inviteCode)
	var id int64
	var expAt int64
	var revoked int
	var boundDevice sql.NullString
	if err = tx.QueryRow(
		"SELECT id, exp_at, revoked, bound_device_id FROM invites WHERE code_hash = ?",
		codeHash,
	).Scan(&id, &expAt, &revoked, &boundDevice); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invite not found")
		}
		return fmt.Errorf("query invite failed: %w", err)
	}
	if revoked != 0 {
		return errors.New("invite revoked")
	}
	if expAt <= now {
		return errors.New("invite expired")
	}
	if !boundDevice.Valid || strings.TrimSpace(boundDevice.String) == "" {
		_, err = tx.Exec(`UPDATE invites
			SET bound_device_id = ?, bound_at = ?, last_seen_at = ?
			WHERE id = ? AND (bound_device_id IS NULL OR bound_device_id = '')`,
			deviceID, now, now, id,
		)
		if err != nil {
			return fmt.Errorf("bind device failed: %w", err)
		}
	} else if boundDevice.String != deviceID {
		return errors.New("invite already bound to another device")
	} else {
		if _, err = tx.Exec("UPDATE invites SET last_seen_at = ? WHERE id = ?", now, id); err != nil {
			return fmt.Errorf("update last_seen failed: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

func (s *InviteStore) validateByDevice(deviceID string, now int64) error {
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM invites
		WHERE bound_device_id = ? AND revoked = 0 AND exp_at > ?
		ORDER BY bound_at DESC LIMIT 1`, deviceID, now).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("device not bound to any valid invite")
		}
		return fmt.Errorf("query by device failed: %w", err)
	}
	if _, err := s.db.Exec("UPDATE invites SET last_seen_at = ? WHERE id = ?", now, id); err != nil {
		return fmt.Errorf("update last_seen failed: %w", err)
	}
	return nil
}

func generateInviteCode(length int) (string, error) {
	if length <= 0 {
		length = inviteCodeLength
	}
	const digits = "0123456789"
	var b strings.Builder
	b.Grow(length)
	max := big.NewInt(int64(len(digits)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("rand failed: %w", err)
		}
		b.WriteByte(digits[n.Int64()])
	}
	return b.String(), nil
}

func hashInviteCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "unique")
}
