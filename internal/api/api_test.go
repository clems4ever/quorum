package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quorum/internal/model"
	"quorum/internal/store"
)

var (
	t0   = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	ada  = model.User{ID: "u1", Email: "ada@example.com", Name: "Ada"}
	alan = model.User{ID: "u2", Email: "alan@example.com", Name: "Alan"}
)

// headerAuth authenticates requests with the X-User header.
type headerAuth struct {
	users map[string]model.User
	err   error
}

func (h headerAuth) User(r *http.Request) (model.User, error) {
	if h.err != nil {
		return model.User{}, h.err
	}
	u, ok := h.users[r.Header.Get("X-User")]
	if !ok {
		return u, model.ErrNotFound
	}
	return u, nil
}

// failing wraps a Store and makes selected methods fail.
type failing struct {
	Store
	fail map[string]bool
}

var errDB = errors.New("db down")

func (f *failing) check(name string) error {
	if f.fail[name] {
		return errDB
	}
	return nil
}

func (f *failing) CreateEvent(ctx context.Context, e model.Event) error {
	if err := f.check("CreateEvent"); err != nil {
		return err
	}
	return f.Store.CreateEvent(ctx, e)
}

func (f *failing) UpdateEvent(ctx context.Context, e model.Event) error {
	if err := f.check("UpdateEvent"); err != nil {
		return err
	}
	return f.Store.UpdateEvent(ctx, e)
}

func (f *failing) SetClosed(ctx context.Context, id string, c bool, fs string, now time.Time) error {
	if err := f.check("SetClosed"); err != nil {
		return err
	}
	return f.Store.SetClosed(ctx, id, c, fs, now)
}

func (f *failing) DeleteEvent(ctx context.Context, id string) error {
	if err := f.check("DeleteEvent"); err != nil {
		return err
	}
	return f.Store.DeleteEvent(ctx, id)
}

func (f *failing) GetEvent(ctx context.Context, id string) (model.EventDetail, error) {
	if err := f.check("GetEvent"); err != nil {
		return model.EventDetail{}, err
	}
	return f.Store.GetEvent(ctx, id)
}

func (f *failing) ListEvents(ctx context.Context, uid string) ([]model.EventSummary, error) {
	if err := f.check("ListEvents"); err != nil {
		return nil, err
	}
	return f.Store.ListEvents(ctx, uid)
}

func (f *failing) SetResponse(ctx context.Context, eid, uid string, v map[string]model.Vote, now time.Time) error {
	if err := f.check("SetResponse"); err != nil {
		return err
	}
	return f.Store.SetResponse(ctx, eid, uid, v, now)
}

func (f *failing) DeleteResponse(ctx context.Context, eid, uid string) error {
	if err := f.check("DeleteResponse"); err != nil {
		return err
	}
	return f.Store.DeleteResponse(ctx, eid, uid)
}

type env struct {
	t     *testing.T
	mux   *http.ServeMux
	api   *API
	store *failing
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, u := range []model.User{ada, alan} {
		if err := db.UpsertUser(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	f := &failing{Store: db, fail: map[string]bool{}}
	a := &API{
		Store: f,
		Auth:  headerAuth{users: map[string]model.User{"ada": ada, "alan": alan}},
		Now:   func() time.Time { return t0 },
		NewID: func() (string, error) {
			n++
			return fmt.Sprintf("id%d", n), nil
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	a.Register(mux)
	return &env{t: t, mux: mux, api: a, store: f}
}

func (e *env) do(user, method, target string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		data, _ := json.Marshal(b)
		r = strings.NewReader(string(data))
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, r)
	if user != "" {
		req.Header.Set("X-User", user)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, req)
	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, msg string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body)
	}
	if msg != "" && !strings.Contains(w.Body.String(), msg) {
		t.Fatalf("body %q does not contain %q", w.Body, msg)
	}
}

func decodeEvent(t *testing.T, w *httptest.ResponseRecorder) model.EventDetail {
	t.Helper()
	var d model.EventDetail
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func hour(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }

func validInput() map[string]any {
	return map[string]any{
		"title": "  Team lunch ", "description": "Let's eat", "location": "Cafe", "mode": "hours",
		"slots": []map[string]any{{"start": hour(1), "end": hour(2)}, {"start": hour(3), "end": hour(4)}},
	}
}

func (e *env) create() model.EventDetail {
	e.t.Helper()
	w := e.do("ada", "POST", "/api/events", validInput())
	expect(e.t, w, http.StatusCreated, "")
	return decodeEvent(e.t, w)
}

func TestMeAndAuth(t *testing.T) {
	e := newEnv(t)
	expect(t, e.do("", "GET", "/api/me", nil), 401, `{"code":"not_signed_in","error":"not signed in"}`)
	w := e.do("ada", "GET", "/api/me", nil)
	expect(t, w, 200, `"email":"ada@example.com"`)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("API responses must not be cached")
	}
	e.api.Auth = headerAuth{err: errDB}
	expect(t, e.do("ada", "GET", "/api/me", nil), 500, "internal error")
}

func TestUnknownRoute(t *testing.T) {
	e := newEnv(t)
	expect(t, e.do("ada", "GET", "/api/nope", nil), 404, "not found")
}

func TestCreateAndGet(t *testing.T) {
	e := newEnv(t)
	d := e.create()
	if d.ID != "id1" || d.Title != "Team lunch" || d.Owner.ID != ada.ID || len(d.Slots) != 2 || d.Slots[0].ID != "id2" {
		t.Fatalf("created: %+v", d)
	}
	w := e.do("alan", "GET", "/api/events/id1", nil)
	expect(t, w, 200, "")
	if got := decodeEvent(t, w); got.Title != "Team lunch" {
		t.Fatalf("get: %+v", got)
	}
	expect(t, e.do("alan", "GET", "/api/events/nope", nil), 404, "event not found")
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t)
	slots := func(n int) []map[string]any {
		s := make([]map[string]any, n)
		for i := range s {
			s[i] = map[string]any{"start": hour(i), "end": hour(i + 1)}
		}
		return s
	}
	cases := map[string]struct {
		mutate func(map[string]any)
		msg    string
	}{
		"title":       {func(m map[string]any) { m["title"] = "  " }, "title is required"},
		"long title":  {func(m map[string]any) { m["title"] = strings.Repeat("a", MaxTitle+1) }, "title is too long"},
		"description": {func(m map[string]any) { m["description"] = strings.Repeat("a", MaxDescription+1) }, "description is too long"},
		"location":    {func(m map[string]any) { m["location"] = strings.Repeat("a", MaxLocation+1) }, "location is too long"},
		"no slots":    {func(m map[string]any) { m["slots"] = slots(0) }, "at least one slot"},
		"many slots":  {func(m map[string]any) { m["slots"] = slots(MaxSlots + 1) }, "too many slots"},
		"bad range":   {func(m map[string]any) { m["slots"] = []map[string]any{{"start": hour(2), "end": hour(1)}} }, "end after"},
		"zero start":  {func(m map[string]any) { m["slots"] = []map[string]any{{"end": hour(1)}} }, "end after"},
		"duplicate": {func(m map[string]any) {
			m["slots"] = []map[string]any{{"start": hour(1), "end": hour(2)}, {"start": hour(1), "end": hour(2)}}
		}, "unique"},
		"mode":    {func(m map[string]any) { m["mode"] = "weeks" }, "mode must be"},
		"unknown": {func(m map[string]any) { m["extra"] = 1 }, "invalid JSON"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			c.mutate(in)
			expect(t, e.do("ada", "POST", "/api/events", in), 400, c.msg)
		})
	}
	expect(t, e.do("ada", "POST", "/api/events", "{"), 400, "invalid JSON")
}

func TestCreateFailures(t *testing.T) {
	e := newEnv(t)
	e.store.fail["CreateEvent"] = true
	expect(t, e.do("ada", "POST", "/api/events", validInput()), 500, "")

	e = newEnv(t)
	e.store.fail["GetEvent"] = true
	expect(t, e.do("ada", "POST", "/api/events", validInput()), 500, "")

	e = newEnv(t)
	e.api.NewID = func() (string, error) { return "", errDB }
	expect(t, e.do("ada", "POST", "/api/events", validInput()), 500, "")

	e = newEnv(t)
	calls := 0
	e.api.NewID = func() (string, error) {
		calls++
		if calls > 1 {
			return "", errDB
		}
		return "ev", nil
	}
	expect(t, e.do("ada", "POST", "/api/events", validInput()), 500, "")
}

func TestList(t *testing.T) {
	e := newEnv(t)
	e.create()
	w := e.do("ada", "GET", "/api/events", nil)
	expect(t, w, 200, `"role":"owner"`)
	var list []model.EventSummary
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SlotCount != 2 {
		t.Fatalf("list: %+v", list)
	}
	expect(t, e.do("alan", "GET", "/api/events", nil), 200, "[]")
	e.store.fail["ListEvents"] = true
	expect(t, e.do("ada", "GET", "/api/events", nil), 500, "")
}

func TestUpdate(t *testing.T) {
	e := newEnv(t)
	d := e.create()
	keep := d.Slots[1]
	in := map[string]any{
		"title": "Dinner", "description": "", "location": "Home",
		"slots": []map[string]any{
			{"id": keep.ID, "start": keep.Start, "end": keep.End},
			{"id": "forged", "start": hour(7), "end": hour(8)},
		},
	}
	w := e.do("ada", "PUT", "/api/events/id1", in)
	expect(t, w, 200, "")
	got := decodeEvent(t, w)
	if got.Title != "Dinner" || len(got.Slots) != 2 || got.Slots[0].ID != keep.ID || got.Slots[1].ID == "forged" {
		t.Fatalf("updated: %+v", got)
	}

	expect(t, e.do("alan", "PUT", "/api/events/id1", in), 403, "organizer")
	expect(t, e.do("ada", "PUT", "/api/events/nope", in), 404, "")
	in["mode"] = "days"
	expect(t, e.do("ada", "PUT", "/api/events/id1", in), 400, "cannot be changed")
	in["mode"] = "hours"
	expect(t, e.do("ada", "PUT", "/api/events/id1", "{"), 400, "invalid JSON")
	in["title"] = ""
	expect(t, e.do("ada", "PUT", "/api/events/id1", in), 400, "title")
}

func TestUpdateReusingIDTwice(t *testing.T) {
	e := newEnv(t)
	d := e.create()
	id := d.Slots[0].ID
	in := map[string]any{"title": "x", "slots": []map[string]any{
		{"id": id, "start": hour(1), "end": hour(2)},
		{"id": id, "start": hour(5), "end": hour(6)},
	}}
	w := e.do("ada", "PUT", "/api/events/id1", in)
	expect(t, w, 200, "")
	got := decodeEvent(t, w)
	if len(got.Slots) != 2 || got.Slots[0].ID == got.Slots[1].ID {
		t.Fatalf("slots: %+v", got.Slots)
	}
}

func TestUpdateFailures(t *testing.T) {
	in := validInput()
	e := newEnv(t)
	e.create()
	e.store.fail["UpdateEvent"] = true
	expect(t, e.do("ada", "PUT", "/api/events/id1", in), 500, "")

	e = newEnv(t)
	e.create()
	e.store.fail["GetEvent"] = true
	expect(t, e.do("ada", "PUT", "/api/events/id1", in), 500, "")

	e = newEnv(t)
	e.create()
	e.api.NewID = func() (string, error) { return "", errDB }
	expect(t, e.do("ada", "PUT", "/api/events/id1", in), 500, "")
}

func TestDelete(t *testing.T) {
	e := newEnv(t)
	e.create()
	expect(t, e.do("alan", "DELETE", "/api/events/id1", nil), 403, "")
	e.store.fail["DeleteEvent"] = true
	expect(t, e.do("ada", "DELETE", "/api/events/id1", nil), 500, "")
	e.store.fail["DeleteEvent"] = false
	expect(t, e.do("ada", "DELETE", "/api/events/id1", nil), 204, "")
	expect(t, e.do("ada", "GET", "/api/events/id1", nil), 404, "")
}

func TestCloseAndReopen(t *testing.T) {
	e := newEnv(t)
	d := e.create()
	expect(t, e.do("alan", "POST", "/api/events/id1/close", nil), 403, "")
	expect(t, e.do("ada", "POST", "/api/events/id1/close", `{"finalSlotId":"nope"}`), 400, "unknown slot")
	expect(t, e.do("ada", "POST", "/api/events/id1/close", `{`), 400, "invalid JSON")

	w := e.do("ada", "POST", "/api/events/id1/close", map[string]string{"finalSlotId": d.Slots[0].ID})
	expect(t, w, 200, "")
	if got := decodeEvent(t, w); !got.Closed || got.FinalSlotID != d.Slots[0].ID {
		t.Fatalf("closed: %+v", got)
	}
	expect(t, e.do("alan", "PUT", "/api/events/id1/response", `{"votes":{}}`), 409, `"code":"poll_closed"`)
	expect(t, e.do("alan", "DELETE", "/api/events/id1/response", nil), 409, "closed")

	expect(t, e.do("alan", "POST", "/api/events/id1/reopen", nil), 403, "")
	w = e.do("ada", "POST", "/api/events/id1/reopen", nil)
	expect(t, w, 200, "")
	if got := decodeEvent(t, w); got.Closed || got.FinalSlotID != "" {
		t.Fatalf("reopened: %+v", got)
	}

	// Closing without a final slot works too.
	expect(t, e.do("ada", "POST", "/api/events/id1/close", nil), 200, `"closed":true`)

	e.store.fail["SetClosed"] = true
	expect(t, e.do("ada", "POST", "/api/events/id1/reopen", nil), 500, "")
}

func TestVoting(t *testing.T) {
	e := newEnv(t)
	d := e.create()
	a, b := d.Slots[0].ID, d.Slots[1].ID
	w := e.do("alan", "PUT", "/api/events/id1/response", map[string]any{"votes": map[string]string{a: "yes", b: "maybe"}})
	expect(t, w, 200, "")
	got := decodeEvent(t, w)
	if len(got.Responses) != 1 || got.Responses[0].Votes[a] != model.VoteYes || got.Responses[0].Votes[b] != model.VoteMaybe {
		t.Fatalf("responses: %+v", got.Responses)
	}

	w = e.do("alan", "PUT", "/api/events/id1/response", map[string]any{"votes": map[string]string{a: "no", b: ""}})
	expect(t, w, 200, "")
	if got := decodeEvent(t, w); len(got.Responses[0].Votes) != 0 {
		t.Fatalf("no votes expected: %+v", got.Responses)
	}

	// Alan now sees the event in his list.
	expect(t, e.do("alan", "GET", "/api/events", nil), 200, `"role":"participant"`)

	expect(t, e.do("alan", "PUT", "/api/events/id1/response", map[string]any{"votes": map[string]string{"zz": "yes"}}), 400, "unknown slot")
	expect(t, e.do("alan", "PUT", "/api/events/id1/response", map[string]any{"votes": map[string]string{a: "perhaps"}}), 400, "votes must be")
	expect(t, e.do("alan", "PUT", "/api/events/id1/response", "{"), 400, "invalid JSON")
	expect(t, e.do("alan", "PUT", "/api/events/nope/response", `{"votes":{}}`), 404, "")

	w = e.do("alan", "DELETE", "/api/events/id1/response", nil)
	expect(t, w, 200, "")
	if got := decodeEvent(t, w); len(got.Responses) != 0 {
		t.Fatalf("response should be withdrawn: %+v", got.Responses)
	}
	expect(t, e.do("alan", "DELETE", "/api/events/nope/response", nil), 404, "")

	e.store.fail["SetResponse"] = true
	e.store.fail["DeleteResponse"] = true
	expect(t, e.do("alan", "PUT", "/api/events/id1/response", `{"votes":{}}`), 500, "")
	expect(t, e.do("alan", "DELETE", "/api/events/id1/response", nil), 500, "")
}

// GetEvent may fail with ErrNotFound after a mutation if the event was
// concurrently deleted.
type vanishing struct {
	Store
	calls int
}

func (v *vanishing) GetEvent(ctx context.Context, id string) (model.EventDetail, error) {
	v.calls++
	if v.calls > 1 {
		return model.EventDetail{}, model.ErrNotFound
	}
	return v.Store.GetEvent(ctx, id)
}

func TestEventVanishesAfterMutation(t *testing.T) {
	e := newEnv(t)
	e.create()
	e.api.Store = &vanishing{Store: e.store}
	expect(t, e.do("ada", "POST", "/api/events/id1/reopen", nil), 404, "")
}
