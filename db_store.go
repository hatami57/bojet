// Package bojet provides a SQLite-backed UserStore for bojet, built on
// microjet's database stack: the same pure-Go glebarez/sqlite driver and the
// generic gormx.Table helpers. Because it shares microjet's driver, a host
// application can hand its own *gorm.DB to NewWithDB and the bot will store its
// users in that same database/connection — no second driver, no second handle.
package bojet

import (
	"context"
	"time"

	"github.com/hatami57/microjet/gormx"
	"github.com/hatami57/microjet/host"
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
func (userRecord) TableName() string { return "telegram_bot_users" }

func toRecord(u *User) *userRecord {
	return &userRecord{
		TgID:        u.ID,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Username:    u.Username,
		PhoneNumber: u.PhoneNumber,
		IsConfirmed: u.IsConfirmed,
	}
}

func (r *userRecord) toUser() *User {
	return &User{
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
type dbStore struct {
	gormx.BaseRepository
	db    *gorm.DB
	users *gormx.Table[userRecord]
}

// NewWithDB wraps an existing *gorm.DB — typically the one microjet's host
// already opened (app.DB()) — migrates the users schema, and returns a Store
// that shares that connection with the rest of the application. The caller
// retains ownership of db: Store.Close leaves it open.
//
//	store, _ := sqlite.NewWithDB(app.DB())
func NewDBStore() UserStore {
	return &dbStore{}
}

func (d *dbStore) Init(app *host.App) error {
	base := gormx.NewBaseRepository(app.DB())
	d.BaseRepository = base
	d.db = app.DB()
	d.users = gormx.NewTableFor[userRecord](&base)
	return nil
}

func (d *dbStore) Setup(app *host.App) error {
	return app.DB().AutoMigrate(&userRecord{})
}

// Close do nothing here because the database is external and managed elsewhere.
func (d *dbStore) Close() error {
	return nil
}

// GetUser returns the user with the given Telegram ID, or nil if not found.
func (d *dbStore) GetUser(id int64) (*User, error) {
	rec, err := d.users.Find(context.Background(), id)
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
func (d *dbStore) SaveUser(u *User) error {
	return d.db.WithContext(context.Background()).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tg_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"first_name", "last_name", "username", "phone_number"}),
		}).
		Create(toRecord(u)).Error
}

// SetConfirmed updates the is_confirmed flag for the given user.
func (d *dbStore) SetConfirmed(id int64, confirmed bool) error {
	return d.users.UpdateMap(context.Background(),
		map[string]any{"is_confirmed": confirmed}, "tg_id = ?", id)
}

// ListConfirmedIDs returns the Telegram IDs of all confirmed users.
func (d *dbStore) ListConfirmedIDs() ([]int64, error) {
	var ids []int64
	err := d.users.PluckDistinct(context.Background(), "tg_id", &ids, "is_confirmed = ?", true)
	return ids, err
}
