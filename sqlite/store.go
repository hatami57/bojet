// Package sqlite provides a SQLite-backed UserStore for bojet, built on
// microjet's database stack: the same pure-Go glebarez/sqlite driver and the
// generic gormx.Table helpers. Because it shares microjet's driver, a host
// application can hand its own *gorm.DB to NewWithDB and the bot will store its
// users in that same database/connection — no second driver, no second handle.
package sqlite

import (
	"context"
	"errors"
	"time"

	"bojet"

	"github.com/glebarez/sqlite"
	"github.com/hatami57/microjet/gormx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// userRecord is the persisted shape of a bojet.User. Runtime-only fields of
// bojet.User (CurrentPage, PageHistory, …) are not stored. The Telegram user ID
// is the primary key, matching bojet.User.ID.
type userRecord struct {
	TgID        int64  `gorm:"column:tg_id;primaryKey"`
	FirstName   string `gorm:"not null;default:''"`
	LastName    string `gorm:"not null;default:''"`
	Username    string `gorm:"not null;default:''"`
	PhoneNumber string `gorm:"not null;default:''"`
	IsConfirmed bool   `gorm:"not null;default:false"`
	CreatedAt   time.Time
}

// TableName keeps the historical "users" table name.
func (userRecord) TableName() string { return "users" }

func toRecord(u *bojet.User) *userRecord {
	return &userRecord{
		TgID:        u.ID,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Username:    u.Username,
		PhoneNumber: u.PhoneNumber,
		IsConfirmed: u.IsConfirmed,
	}
}

func (r *userRecord) toUser() *bojet.User {
	return &bojet.User{
		ID:          r.TgID,
		FirstName:   r.FirstName,
		LastName:    r.LastName,
		Username:    r.Username,
		PhoneNumber: r.PhoneNumber,
		IsConfirmed: r.IsConfirmed,
	}
}

// Store is a SQLite-backed implementation of bojet.UserStore, backed by a
// gormx.Table over microjet's GORM connection.
type Store struct {
	db    *gorm.DB
	users *gormx.Table[userRecord]
	// external is true when db was supplied by the caller (NewWithDB); in that
	// case the caller owns the connection and Close must not close it.
	external bool
}

// NewStore opens (or creates) a SQLite database at the given path using the same
// glebarez/sqlite driver microjet uses, migrates the schema, and returns a Store
// ready to pass to bojet.WithStore(). Store.Close closes the connection.
func NewStore(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	store, err := NewWithDB(db)
	if err != nil {
		if sqlDB, dErr := db.DB(); dErr == nil {
			sqlDB.Close()
		}
		return nil, err
	}
	store.external = false
	return store, nil
}

// NewWithDB wraps an existing *gorm.DB — typically the one microjet's host
// already opened (app.DB()) — migrates the users schema, and returns a Store
// that shares that connection with the rest of the application. The caller
// retains ownership of db: Store.Close leaves it open.
//
//	store, _ := sqlite.NewWithDB(app.DB())
func NewWithDB(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqlite: db is nil")
	}
	if err := db.AutoMigrate(&userRecord{}); err != nil {
		return nil, err
	}
	return &Store{db: db, users: gormx.NewTable[userRecord](db), external: true}, nil
}

// Close closes the underlying database connection, unless it was supplied by the
// caller via NewWithDB (in which case Close is a no-op and the caller closes it).
func (s *Store) Close() error {
	if s.external {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetUser returns the user with the given Telegram ID, or nil if not found.
func (s *Store) GetUser(id int64) (*bojet.User, error) {
	rec, err := s.users.Find(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return rec.toUser(), nil
}

// SaveUser inserts the user, or on Telegram-ID conflict updates only the profile
// fields — is_confirmed and created_at are preserved across re-registration.
func (s *Store) SaveUser(u *bojet.User) error {
	return s.db.WithContext(context.Background()).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tg_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"first_name", "last_name", "username", "phone_number"}),
		}).
		Create(toRecord(u)).Error
}

// SetConfirmed updates the is_confirmed flag for the given user.
func (s *Store) SetConfirmed(id int64, confirmed bool) error {
	return s.users.UpdateMap(context.Background(),
		map[string]any{"is_confirmed": confirmed}, "tg_id = ?", id)
}

// ListConfirmedIDs returns the Telegram IDs of all confirmed users.
func (s *Store) ListConfirmedIDs() ([]int64, error) {
	var ids []int64
	err := s.users.PluckDistinct(context.Background(), "tg_id", &ids, "is_confirmed = ?", true)
	return ids, err
}
