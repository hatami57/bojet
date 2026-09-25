package bojet

import "sync"

// keyedMutex is a set of mutexes keyed by Telegram user ID. telebot runs every
// update in its own goroutine, so the bot takes the sender's lock for the
// duration of a handler: updates from one user are processed one at a time
// (their Session is not safe for concurrent use), while different users still
// run in parallel. Entries are reference-counted and dropped once unused, so
// the map only holds users with an update in flight.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[int64]*keyedEntry
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

// lock acquires the mutex for id and returns the function that releases it.
func (k *keyedMutex) lock(id int64) (unlock func()) {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = map[int64]*keyedEntry{}
	}
	e := k.locks[id]
	if e == nil {
		e = &keyedEntry{}
		k.locks[id] = e
	}
	e.refs++
	k.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(k.locks, id)
		}
		k.mu.Unlock()
	}
}

// relayLogCap bounds how many forwarded contact-admin messages are remembered
// for routing admin replies. The oldest entries are dropped first.
const relayLogCap = 10_000

// relayKey identifies a message in an admin's chat.
type relayKey struct {
	chatID int64
	msgID  int
}

// relayLog remembers which user each contact-admin message forwarded to an
// admin came from. It lets an admin's reply reach the user even when Telegram
// hides the original sender of the forward (users who restrict forwarding).
type relayLog struct {
	mu    sync.Mutex
	users map[relayKey]int64
	order []relayKey
}

func (r *relayLog) add(chatID int64, msgID int, userID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.users == nil {
		r.users = map[relayKey]int64{}
	}
	k := relayKey{chatID: chatID, msgID: msgID}
	if _, ok := r.users[k]; !ok {
		r.order = append(r.order, k)
	}
	r.users[k] = userID
	for len(r.order) > relayLogCap {
		delete(r.users, r.order[0])
		r.order = r.order[1:]
	}
}

func (r *relayLog) lookup(chatID int64, msgID int) (int64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.users[relayKey{chatID: chatID, msgID: msgID}]
	return id, ok
}
