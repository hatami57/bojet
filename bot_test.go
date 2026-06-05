package bojet

import (
	"testing"
	"time"

	"github.com/hatami57/microjet/core"
)

// fakeClock is a controllable core.TimeProvider for tests.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) NowTS() int64          { return c.now.Unix() }
func (c *fakeClock) NowSortable() string   { return core.TimeToSortable(c.now) }
func (c *fakeClock) NowSortableMS() string { return core.TimeToSortableMS(c.now) }

// countingStore records how many times GetUser is called so we can assert
// whether a resolveUser hit the cache or fell through to the store.
type countingStore struct {
	user  *User
	loads int
}

func (s *countingStore) GetUser(int64) (*User, error) { s.loads++; return s.user, nil }
func (s *countingStore) SaveUser(*User) error         { return nil }
func (s *countingStore) SetConfirmed(int64, bool) error {
	return nil
}
func (s *countingStore) ListConfirmedIDs() ([]int64, error) { return nil, nil }

func TestResolveUserCacheExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	store := &countingStore{user: &User{ID: 1, FirstName: "Ada"}}

	b := &Bot{
		store:       store,
		users:       map[int64]*User{},
		cacheExpiry: 30 * time.Minute,
		clock:       clock,
	}

	// First resolution loads from the store.
	if _, err := b.resolveUser(1); err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if store.loads != 1 {
		t.Fatalf("first resolve: got %d loads, want 1", store.loads)
	}

	// Within the TTL the cached entry is reused (no extra store load).
	clock.now = clock.now.Add(20 * time.Minute)
	if _, err := b.resolveUser(1); err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if store.loads != 1 {
		t.Fatalf("cached resolve: got %d loads, want 1", store.loads)
	}

	// Past the TTL the entry is stale and reloaded from the store.
	clock.now = clock.now.Add(31 * time.Minute)
	if _, err := b.resolveUser(1); err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if store.loads != 2 {
		t.Fatalf("expired resolve: got %d loads, want 2", store.loads)
	}
}
