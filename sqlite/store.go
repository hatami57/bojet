// Package sqlite provides a SQLite-backed UserStore for bojet.
package sqlite

import (
	"bojet"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    tg_id        BIGINT UNIQUE NOT NULL,
    first_name   TEXT NOT NULL DEFAULT '',
    last_name    TEXT NOT NULL DEFAULT '',
    username     TEXT NOT NULL DEFAULT '',
    phone_number TEXT NOT NULL DEFAULT '',
    is_confirmed BOOLEAN NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// Store is a SQLite-backed implementation of bojet.UserStore.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) a SQLite database at the given path and
// applies the schema. Returns a Store ready to pass to bojet.WithStore().
func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetUser(id int64) (*bojet.User, error) {
	u := &bojet.User{}
	err := s.db.QueryRow(
		`SELECT tg_id, first_name, last_name, username, phone_number, is_confirmed
		 FROM users WHERE tg_id = ?`, id,
	).Scan(&u.ID, &u.FirstName, &u.LastName, &u.Username, &u.PhoneNumber, &u.IsConfirmed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (s *Store) SaveUser(u *bojet.User) error {
	_, err := s.db.Exec(
		`INSERT INTO users (tg_id, first_name, last_name, username, phone_number, is_confirmed)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tg_id) DO UPDATE SET
		     first_name   = excluded.first_name,
		     last_name    = excluded.last_name,
		     username     = excluded.username,
		     phone_number = excluded.phone_number`,
		u.ID, u.FirstName, u.LastName, u.Username, u.PhoneNumber, u.IsConfirmed,
	)
	return err
}

func (s *Store) SetConfirmed(id int64, confirmed bool) error {
	_, err := s.db.Exec(`UPDATE users SET is_confirmed = ? WHERE tg_id = ?`, confirmed, id)
	return err
}

func (s *Store) ListConfirmedIDs() ([]int64, error) {
	rows, err := s.db.Query(`SELECT tg_id FROM users WHERE is_confirmed = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
