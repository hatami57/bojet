package bojet

import (
	"testing"
	"time"

	"github.com/hatami57/microjet/core"
	"gopkg.in/telebot.v4"
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
	saves int
}

func (s *countingStore) GetUser(int64) (*User, error) { s.loads++; return s.user, nil }
func (s *countingStore) SaveUser(*User) error         { s.saves++; return nil }
func (s *countingStore) SetConfirmed(int64, bool) error {
	return nil
}
func (s *countingStore) ListConfirmedIDs() ([]int64, error) { return nil, nil }

func TestResolveUserCacheExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	store := &countingStore{user: &User{ID: 1, FirstName: "Ada"}}

	b := &Bot{
		userStore: store,
		users:     map[int64]*User{},
		clock:     clock,
		config: Config{
			CacheExpiry: 30 * time.Minute,
		},
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

func TestResolveUserRestoresActiveForm(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	store := &countingStore{user: &User{ID: 1, FirstName: "Ada"}}

	b := &Bot{
		userStore: store,
		users:     map[int64]*User{},
		clock:     clock,
		config: Config{
			CacheExpiry: 30 * time.Minute,
		},
		sessions: NewMemorySessionStore(),
	}

	// Resolve once, then simulate an in-progress form being persisted.
	u, err := b.resolveUser(1)
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	u.Session.input = &formState{pending: &Question{Key: "q1"}}
	b.saveSession(u)

	// After the user cache expires, the reloaded user resumes the same form.
	clock.now = clock.now.Add(31 * time.Minute)
	u2, err := b.resolveUser(1)
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	fs, ok := u2.Session.input.(*formState)
	if !ok {
		t.Fatalf("active form was not restored after cache expiry: input=%T", u2.Session.input)
	}
	if fs.pending == nil || fs.pending.Key != "q1" {
		t.Fatalf("restored form has wrong pending question: %+v", fs.pending)
	}

	// Once the session is deleted (form completed/cancelled), a reload starts fresh.
	b.deleteSession(1)
	clock.now = clock.now.Add(31 * time.Minute)
	u3, err := b.resolveUser(1)
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if u3.Session.input != nil {
		t.Fatal("expected fresh session after deletion, got an active input state")
	}
}

func TestProvisionCreatesConfirmedUser(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	store := &countingStore{user: nil} // sender is unknown to the store

	b := &Bot{
		userStore: store,
		users:     map[int64]*User{},
		clock:     clock,
		config: Config{
			CacheExpiry: 30 * time.Minute,
		},
		sessions:     NewMemorySessionStore(),
		registration: &NoRegistrationFlow{},
		errorHandler: func(error, telebot.Context) {},
	}

	flow := &NoRegistrationFlow{}
	u, err := b.provision(flow, &telebot.User{ID: 7, FirstName: "Grace", Username: "grace"})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if u == nil {
		t.Fatal("provision returned nil user")
	}
	if u.ID != 7 || !u.IsConfirmed {
		t.Fatalf("provisioned user = %+v; want ID 7, confirmed", u)
	}
	if u.Session == nil {
		t.Fatal("provisioned user has no session")
	}
	if store.saves != 1 {
		t.Fatalf("SaveUser called %d times; want 1", store.saves)
	}
	if _, ok := b.users[7]; !ok {
		t.Fatal("provisioned user was not cached")
	}
}
