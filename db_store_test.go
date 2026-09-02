package bojet

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hatami57/microjet/gormx"
	"github.com/hatami57/microjet/gormx/sqlite"
	"gorm.io/gorm"
)

// newTestStore returns a dbStore over a throwaway in-memory SQLite database,
// wired the same way host init does but without a host.
func newTestStore(t *testing.T) (*dbStore, *gorm.DB) {
	t.Helper()

	db, err := sqlite.Driver().Open(
		gormx.Config{Name: ":memory:", LogLevel: "silent"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	base := gormx.NewBaseRepository(db)
	return &dbStore{
		BaseRepository: base,
		db:             db,
		users:          gormx.NewTableFor[userRecord](&base),
	}, db
}

func TestSetConfirmedReportsUnknownUser(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.SetConfirmed(404, true)
	if err == nil {
		t.Fatal("SetConfirmed on an unknown user returned nil; want a not-found error")
	}
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetConfirmed error = %v; want it to match ErrUserNotFound", err)
	}
}

func TestSetConfirmedUpdatesExistingUser(t *testing.T) {
	store, _ := newTestStore(t)

	if err := store.SaveUser(&User{ID: 7, FirstName: "Grace"}); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	if err := store.SetConfirmed(7, true); err != nil {
		t.Fatalf("SetConfirmed: %v", err)
	}

	u, err := store.GetUser(7)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u == nil || !u.IsConfirmed {
		t.Fatalf("GetUser = %+v; want a confirmed user", u)
	}
}

// SaveUser upserts profile fields only: a re-registration must not silently
// revoke an approval, which is why it hand-rolls ON CONFLICT instead of using
// gormx.Upsert (which updates every column).
func TestSaveUserPreservesConfirmation(t *testing.T) {
	store, _ := newTestStore(t)

	if err := store.SaveUser(&User{ID: 7, FirstName: "Grace"}); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	if err := store.SetConfirmed(7, true); err != nil {
		t.Fatalf("SetConfirmed: %v", err)
	}
	if err := store.SaveUser(&User{ID: 7, FirstName: "Grace", LastName: "Hopper"}); err != nil {
		t.Fatalf("SaveUser (re-register): %v", err)
	}

	u, err := store.GetUser(7)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.LastName != "Hopper" {
		t.Fatalf("LastName = %q; want the re-registered %q", u.LastName, "Hopper")
	}
	if !u.IsConfirmed {
		t.Fatal("re-registration cleared IsConfirmed; approval must survive a profile update")
	}
}

func TestGetUserUnknownReturnsNil(t *testing.T) {
	store, _ := newTestStore(t)

	u, err := store.GetUser(404)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u != nil {
		t.Fatalf("GetUser = %+v; want nil for an unknown user", u)
	}
}

func TestListConfirmedIDs(t *testing.T) {
	store, _ := newTestStore(t)

	for _, u := range []*User{{ID: 1}, {ID: 2}, {ID: 3}} {
		if err := store.SaveUser(u); err != nil {
			t.Fatalf("SaveUser: %v", err)
		}
	}
	if err := store.SetConfirmed(2, true); err != nil {
		t.Fatalf("SetConfirmed: %v", err)
	}

	ids, err := store.ListConfirmedIDs()
	if err != nil {
		t.Fatalf("ListConfirmedIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("ListConfirmedIDs = %v; want [2]", ids)
	}
}
