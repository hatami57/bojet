package bojet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"gopkg.in/telebot.v4"
)

// apiCall is one request the bot made to the fake Telegram Bot API.
type apiCall struct {
	method string
	params map[string]any
}

func (c apiCall) chatID() int64 {
	id, _ := strconv.ParseInt(fmt.Sprint(c.params["chat_id"]), 10, 64)
	return id
}

func (c apiCall) text() string {
	s, _ := c.params["text"].(string)
	return s
}

// fakeAPI is an in-process Telegram Bot API that records every call and
// answers each with a message in the target chat. respond, if set, may
// override the reply for a call (return "" to use the default).
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	calls   []apiCall
	nextID  int
	respond func(apiCall) string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	api := &fakeAPI{t: t, nextID: 1000}
	api.srv = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.srv.Close)
	return api
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	// Path is /bot<token>/<method>.
	method := r.URL.Path[len("/bottest/"):]
	params := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&params)
	call := apiCall{method: method, params: params}

	a.mu.Lock()
	a.calls = append(a.calls, call)
	a.nextID++
	id := a.nextID
	respond := a.respond
	a.mu.Unlock()

	if respond != nil {
		if body := respond(call); body != "" {
			_, _ = w.Write([]byte(body))
			return
		}
	}
	_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"date":0,"chat":{"id":%d,"type":"private"}}}`,
		id, call.chatID())
}

// sent returns the calls of the given method made so far.
func (a *fakeAPI) sent(method string) []apiCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []apiCall
	for _, c := range a.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

// lastText returns the text of the last sendMessage to chatID.
func (a *fakeAPI) lastText(chatID int64) string {
	msgs := a.sent("sendMessage")
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].chatID() == chatID {
			return msgs[i].text()
		}
	}
	return ""
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// memStore is an in-memory UserStore.
type memStore struct {
	mu    sync.Mutex
	users map[int64]User
	loads int
}

func newMemStore(users ...User) *memStore {
	s := &memStore{users: map[int64]User{}}
	for _, u := range users {
		s.users[u.ID] = u
	}
	return s
}

func (s *memStore) GetUser(id int64) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads++
	u, ok := s.users[id]
	if !ok {
		return nil, nil
	}
	return &u, nil
}

func (s *memStore) SaveUser(u *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := *u
	rec.Session = nil
	if cur, ok := s.users[u.ID]; ok {
		rec.IsConfirmed, rec.IsRejected = cur.IsConfirmed, cur.IsRejected
	}
	s.users[u.ID] = rec
	return nil
}

func (s *memStore) SetConfirmed(id int64, confirmed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[id]; ok {
		u.IsConfirmed, u.IsRejected = confirmed, !confirmed
		s.users[id] = u
	}
	return nil
}

func (s *memStore) ListConfirmedIDs() ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []int64
	for id, u := range s.users {
		if u.IsConfirmed {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *memStore) get(id int64) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	return u, ok
}

// newTestBot builds a fully wired bot talking to api. Updates are processed
// synchronously so each call returns after its handler ran.
func newTestBot(t *testing.T, api *fakeAPI, store UserStore, opts ...Option) *Bot {
	t.Helper()
	return newWiredBot(t, api, store, true, opts...)
}

func newWiredBot(t *testing.T, api *fakeAPI, store UserStore, synchronous bool, opts ...Option) *Bot {
	t.Helper()
	b := New(append([]Option{WithCacheExpiry(30 * time.Minute), WithContactAdmin(true)}, opts...)...)
	b.applyOptions()
	for _, id := range b.config.AdminIDs {
		b.adminIDs[id] = struct{}{}
	}
	tb, err := telebot.NewBot(telebot.Settings{
		URL:         api.srv.URL,
		Token:       "test",
		Offline:     true,
		Synchronous: synchronous,
	})
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	b.tb = tb
	b.userStore = store
	b.buildPublicKeyboard()
	if err := b.wire(); err != nil {
		t.Fatalf("wire: %v", err)
	}
	return b
}

var updateSeq int

// text delivers a private text message from user id.
func text(b *Bot, from int64, s string) {
	updateSeq++
	b.tb.ProcessUpdate(telebot.Update{ID: updateSeq, Message: &telebot.Message{
		ID:     updateSeq,
		Sender: &telebot.User{ID: from, FirstName: fmt.Sprint("user", from)},
		Chat:   &telebot.Chat{ID: from, Type: telebot.ChatPrivate},
		Text:   s,
	}})
}

// message delivers an arbitrary private message from user id.
func message(b *Bot, from int64, m *telebot.Message) {
	updateSeq++
	m.ID = updateSeq
	m.Sender = &telebot.User{ID: from, FirstName: fmt.Sprint("user", from)}
	m.Chat = &telebot.Chat{ID: from, Type: telebot.ChatPrivate}
	b.tb.ProcessUpdate(telebot.Update{ID: updateSeq, Message: m})
}

// callback delivers an inline-button press by admin on button unique with data.
func callback(b *Bot, from int64, unique, data string) {
	updateSeq++
	b.tb.ProcessUpdate(telebot.Update{ID: updateSeq, Callback: &telebot.Callback{
		ID:      strconv.Itoa(updateSeq),
		Sender:  &telebot.User{ID: from},
		Data:    "\f" + unique + "|" + data,
		Message: &telebot.Message{ID: 1, Chat: &telebot.Chat{ID: from, Type: telebot.ChatPrivate}},
	}})
}
