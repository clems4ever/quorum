// Package store persists users, sessions, events and votes in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"quorum/internal/model"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT NOT NULL,
	name       TEXT NOT NULL,
	picture    TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	id            TEXT PRIMARY KEY,
	owner_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title         TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	location      TEXT NOT NULL DEFAULT '',
	mode          TEXT NOT NULL,
	closed        INTEGER NOT NULL DEFAULT 0,
	final_slot_id TEXT NOT NULL DEFAULT '',
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS events_owner ON events(owner_id);
CREATE TABLE IF NOT EXISTS slots (
	id       TEXT PRIMARY KEY,
	event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
	start_at INTEGER NOT NULL,
	end_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS slots_event ON slots(event_id);
CREATE TABLE IF NOT EXISTS responses (
	event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (event_id, user_id)
);
CREATE INDEX IF NOT EXISTS responses_user ON responses(user_id);
CREATE TABLE IF NOT EXISTS votes (
	slot_id TEXT NOT NULL REFERENCES slots(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	value   TEXT NOT NULL,
	PRIMARY KEY (slot_id, user_id)
);
`

// SQLite is a Store backed by a SQLite database.
type SQLite struct {
	db *sql.DB
}

// Open opens (and migrates) the SQLite database at path. Use ":memory:" for an
// ephemeral database.
func Open(ctx context.Context, path string) (*SQLite, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite only supports a single writer; a single connection also keeps
	// in-memory databases consistent.
	db.SetMaxOpenConns(1)
	s := &SQLite{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return s, nil
}

func (s *SQLite) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close releases the database.
func (s *SQLite) Close() error { return s.db.Close() }

// Ping checks that the database is reachable.
func (s *SQLite) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func unix(t time.Time) int64     { return t.UTC().Unix() }
func fromUnix(v int64) time.Time { return time.Unix(v, 0).UTC() }

// UpsertUser creates the user or refreshes its profile.
func (s *SQLite) UpsertUser(ctx context.Context, u model.User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, name, picture) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET email = excluded.email, name = excluded.name, picture = excluded.picture`,
		u.ID, u.Email, u.Name, u.Picture)
	return err
}

// CreateSession stores a session for the user, keyed by the token hash.
func (s *SQLite) CreateSession(ctx context.Context, tokenHash, userID string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, userID, unix(expires))
	return err
}

// SessionUser returns the user owning a non-expired session.
func (s *SQLite) SessionUser(ctx context.Context, tokenHash string, now time.Time) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.picture FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, unix(now)).
		Scan(&u.ID, &u.Email, &u.Name, &u.Picture)
	if errors.Is(err, sql.ErrNoRows) {
		return u, model.ErrNotFound
	}
	return u, err
}

// DeleteSession removes a session.
func (s *SQLite) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// CreateEvent inserts the event and its slots. Slot IDs must be set.
func (s *SQLite) CreateEvent(ctx context.Context, e model.Event) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO events (id, owner_id, title, description, location, mode, closed, final_slot_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, '', ?, ?)`,
			e.ID, e.Owner.ID, e.Title, e.Description, e.Location, string(e.Mode), unix(e.CreatedAt), unix(e.UpdatedAt))
		if err != nil {
			return err
		}
		return insertSlots(ctx, tx, e.ID, e.Slots)
	})
}

func insertSlots(ctx context.Context, tx *sql.Tx, eventID string, slots []model.Slot) error {
	for _, sl := range slots {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO slots (id, event_id, start_at, end_at) VALUES (?, ?, ?, ?)`,
			sl.ID, eventID, unix(sl.Start), unix(sl.End)); err != nil {
			return err
		}
	}
	return nil
}

// UpdateEvent replaces the editable fields and the slot list of an event.
// Slots whose ID is not present anymore are deleted together with their
// votes; the final slot is cleared if it disappeared.
func (s *SQLite) UpdateEvent(ctx context.Context, e model.Event) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE events SET title = ?, description = ?, location = ?, updated_at = ? WHERE id = ?`,
			e.Title, e.Description, e.Location, unix(e.UpdatedAt), e.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return model.ErrNotFound
		}
		existing, err := slotIDs(ctx, tx, e.ID)
		if err != nil {
			return err
		}
		keep := map[string]bool{}
		var added []model.Slot
		for _, sl := range e.Slots {
			if existing[sl.ID] {
				keep[sl.ID] = true
				if _, err := tx.ExecContext(ctx, `UPDATE slots SET start_at = ?, end_at = ? WHERE id = ?`,
					unix(sl.Start), unix(sl.End), sl.ID); err != nil {
					return err
				}
				continue
			}
			added = append(added, sl)
		}
		for id := range existing {
			if keep[id] {
				continue
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM slots WHERE id = ?`, id); err != nil {
				return err
			}
		}
		if err := insertSlots(ctx, tx, e.ID, added); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE events SET final_slot_id = '' WHERE id = ? AND final_slot_id != ''
			AND final_slot_id NOT IN (SELECT id FROM slots WHERE event_id = ?)`, e.ID, e.ID)
		return err
	})
}

func slotIDs(ctx context.Context, tx *sql.Tx, eventID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM slots WHERE event_id = ?`, eventID)
	if err != nil {
		return nil, err
	}
	list, err := collect(rows, func(r *sql.Rows) (id string, err error) {
		err = r.Scan(&id)
		return id, err
	})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(list))
	for _, id := range list {
		ids[id] = true
	}
	return ids, nil
}

// collect scans every row with scan, then closes rows. Errors from the scan,
// the iteration and Close are all reported.
func collect[T any](rows *sql.Rows, scan func(*sql.Rows) (T, error)) (out []T, err error) {
	defer func() { err = errors.Join(err, rows.Close()) }()
	out = []T{}
	for rows.Next() {
		v, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetClosed closes (with an optional final slot) or reopens an event.
func (s *SQLite) SetClosed(ctx context.Context, eventID string, closed bool, finalSlotID string, now time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE events SET closed = ?, final_slot_id = ?, updated_at = ? WHERE id = ?`,
		closed, finalSlotID, unix(now), eventID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// DeleteEvent removes an event and everything attached to it.
func (s *SQLite) DeleteEvent(ctx context.Context, eventID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, eventID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// GetEvent loads an event with its slots and responses.
func (s *SQLite) GetEvent(ctx context.Context, eventID string) (model.EventDetail, error) {
	var d model.EventDetail
	var mode string
	var created, updated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.title, e.description, e.location, e.mode, e.closed, e.final_slot_id, e.created_at, e.updated_at,
		       u.id, u.email, u.name, u.picture
		FROM events e JOIN users u ON u.id = e.owner_id WHERE e.id = ?`, eventID).
		Scan(&d.ID, &d.Title, &d.Description, &d.Location, &mode, &d.Closed, &d.FinalSlotID, &created, &updated,
			&d.Owner.ID, &d.Owner.Email, &d.Owner.Name, &d.Owner.Picture)
	if errors.Is(err, sql.ErrNoRows) {
		return d, model.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Mode = model.Mode(mode)
	d.CreatedAt, d.UpdatedAt = fromUnix(created), fromUnix(updated)
	if d.Slots, err = s.slots(ctx, eventID); err != nil {
		return d, err
	}
	d.Responses, err = s.responses(ctx, eventID)
	return d, err
}

func (s *SQLite) slots(ctx context.Context, eventID string) ([]model.Slot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, start_at, end_at FROM slots WHERE event_id = ? ORDER BY start_at, end_at, id`, eventID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r *sql.Rows) (model.Slot, error) {
		var sl model.Slot
		var start, end int64
		err := r.Scan(&sl.ID, &start, &end)
		sl.Start, sl.End = fromUnix(start), fromUnix(end)
		return sl, err
	})
}

func (s *SQLite) responses(ctx context.Context, eventID string) ([]model.Response, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.name, u.picture, r.updated_at FROM responses r
		JOIN users u ON u.id = r.user_id WHERE r.event_id = ? ORDER BY r.updated_at, u.name`, eventID)
	if err != nil {
		return nil, err
	}
	resps, err := collect(rows, func(row *sql.Rows) (model.Response, error) {
		r := model.Response{Votes: map[string]model.Vote{}}
		var updated int64
		err := row.Scan(&r.User.ID, &r.User.Email, &r.User.Name, &r.User.Picture, &updated)
		r.UpdatedAt = fromUnix(updated)
		return r, err
	})
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(resps))
	for i, r := range resps {
		index[r.User.ID] = i
	}

	type vote struct{ slotID, userID, value string }
	vrows, err := s.db.QueryContext(ctx, `
		SELECT v.slot_id, v.user_id, v.value FROM votes v
		JOIN slots s ON s.id = v.slot_id WHERE s.event_id = ?`, eventID)
	if err != nil {
		return nil, err
	}
	votes, err := collect(vrows, func(row *sql.Rows) (v vote, err error) {
		err = row.Scan(&v.slotID, &v.userID, &v.value)
		return v, err
	})
	if err != nil {
		return nil, err
	}
	for _, v := range votes {
		if i, ok := index[v.userID]; ok {
			resps[i].Votes[v.slotID] = model.Vote(v.value)
		}
	}
	return resps, nil
}

// SetResponse replaces the votes of a user for an event.
func (s *SQLite) SetResponse(ctx context.Context, eventID, userID string, votes map[string]model.Vote, now time.Time) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if err := deleteVotes(ctx, tx, eventID, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO responses (event_id, user_id, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(event_id, user_id) DO UPDATE SET updated_at = excluded.updated_at`,
			eventID, userID, unix(now)); err != nil {
			return err
		}
		// Deterministic insertion order keeps behavior reproducible.
		ids := make([]string, 0, len(votes))
		for id := range votes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `INSERT INTO votes (slot_id, user_id, value) VALUES (?, ?, ?)`,
				id, userID, string(votes[id])); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteResponse withdraws a user's participation in an event.
func (s *SQLite) DeleteResponse(ctx context.Context, eventID, userID string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if err := deleteVotes(ctx, tx, eventID, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM responses WHERE event_id = ? AND user_id = ?`, eventID, userID)
		return err
	})
}

func deleteVotes(ctx context.Context, tx *sql.Tx, eventID, userID string) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM votes WHERE user_id = ? AND slot_id IN (SELECT id FROM slots WHERE event_id = ?)`,
		userID, eventID)
	return err
}

// ListEvents returns the events owned by or answered by the user, most
// recently updated first.
func (s *SQLite) ListEvents(ctx context.Context, userID string) ([]model.EventSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.title, e.location, e.mode, e.closed, e.created_at, e.updated_at,
		       u.id, u.email, u.name, u.picture,
		       (SELECT COUNT(*) FROM slots s WHERE s.event_id = e.id),
		       (SELECT COUNT(*) FROM responses r WHERE r.event_id = e.id),
		       (SELECT MIN(start_at) FROM slots s WHERE s.event_id = e.id),
		       (SELECT MAX(end_at) FROM slots s WHERE s.event_id = e.id)
		FROM events e JOIN users u ON u.id = e.owner_id
		WHERE e.owner_id = ? OR EXISTS (SELECT 1 FROM responses r WHERE r.event_id = e.id AND r.user_id = ?)
		ORDER BY e.updated_at DESC, e.id`, userID, userID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row *sql.Rows) (model.EventSummary, error) {
		var es model.EventSummary
		var mode string
		var created, updated int64
		var first, last sql.NullInt64
		if err := row.Scan(&es.ID, &es.Title, &es.Location, &mode, &es.Closed, &created, &updated,
			&es.Owner.ID, &es.Owner.Email, &es.Owner.Name, &es.Owner.Picture,
			&es.SlotCount, &es.ResponseCount, &first, &last); err != nil {
			return es, err
		}
		es.Mode = model.Mode(mode)
		es.CreatedAt, es.UpdatedAt = fromUnix(created), fromUnix(updated)
		if first.Valid {
			t := fromUnix(first.Int64)
			es.FirstStart = &t
		}
		if last.Valid {
			t := fromUnix(last.Int64)
			es.LastEnd = &t
		}
		es.Role = model.RoleParticipant
		if es.Owner.ID == userID {
			es.Role = model.RoleOwner
		}
		return es, nil
	})
}

func (s *SQLite) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}
