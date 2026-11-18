package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"quorum/internal/model"
)

var (
	ctx  = context.Background()
	t0   = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	ada  = model.User{ID: "u1", Email: "ada@example.com", Name: "Ada", Picture: "p1"}
	alan = model.User{ID: "u2", Email: "alan@example.com", Name: "Alan"}
)

func open(t *testing.T) *SQLite {
	t.Helper()
	s, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mustClose(t, s) })
	for _, u := range []model.User{ada, alan} {
		if err := s.UpsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func mustClose(t *testing.T, s *SQLite) {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Error(err)
	}
}

func slot(id string, h int) model.Slot {
	return model.Slot{ID: id, Start: t0.Add(time.Duration(h) * time.Hour), End: t0.Add(time.Duration(h+1) * time.Hour)}
}

func event(id string) model.Event {
	return model.Event{
		ID: id, Title: "Lunch", Description: "d", Location: "Paris", Mode: model.ModeHours,
		Owner: ada, Slots: []model.Slot{slot(id+"-b", 2), slot(id+"-a", 1)}, CreatedAt: t0, UpdatedAt: t0,
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenFileAndPing(t *testing.T) {
	s, err := Open(ctx, filepath.Join(t.TempDir(), "x.db"))
	must(t, err)
	defer mustClose(t, s)
	must(t, s.Ping(ctx))
}

func TestOpenFailsOnBadPath(t *testing.T) {
	if _, err := Open(ctx, filepath.Join(t.TempDir(), "missing", "x.db")); err == nil {
		t.Fatal("expected error")
	}
}

func TestUsersAndSessions(t *testing.T) {
	s := open(t)
	updated := ada
	updated.Name = "Ada L."
	must(t, s.UpsertUser(ctx, updated))
	must(t, s.CreateSession(ctx, "h1", ada.ID, t0.Add(time.Hour)))

	u, err := s.SessionUser(ctx, "h1", t0)
	must(t, err)
	if u != updated {
		t.Fatalf("got %+v", u)
	}
	if _, err := s.SessionUser(ctx, "h1", t0.Add(2*time.Hour)); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
	must(t, s.DeleteSession(ctx, "h1"))
	if _, err := s.SessionUser(ctx, "h1", t0); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("deleted session: %v", err)
	}
	if err := s.CreateSession(ctx, "h2", "nobody", t0); err == nil {
		t.Fatal("expected foreign key error")
	}
}

func TestEventLifecycle(t *testing.T) {
	s := open(t)
	must(t, s.CreateEvent(ctx, event("e1")))

	d, err := s.GetEvent(ctx, "e1")
	must(t, err)
	if d.Title != "Lunch" || d.Owner != ada || d.Mode != model.ModeHours || !d.CreatedAt.Equal(t0) {
		t.Fatalf("unexpected event %+v", d)
	}
	if len(d.Slots) != 2 || d.Slots[0].ID != "e1-a" || d.Slots[1].ID != "e1-b" {
		t.Fatalf("slots not sorted: %+v", d.Slots)
	}
	if len(d.Responses) != 0 {
		t.Fatalf("responses: %+v", d.Responses)
	}

	// Votes.
	must(t, s.SetResponse(ctx, "e1", alan.ID, map[string]model.Vote{"e1-a": model.VoteYes, "e1-b": model.VoteMaybe}, t0))
	must(t, s.SetResponse(ctx, "e1", ada.ID, map[string]model.Vote{}, t0.Add(time.Minute)))
	must(t, s.SetResponse(ctx, "e1", alan.ID, map[string]model.Vote{"e1-b": model.VoteYes}, t0))
	d, _ = s.GetEvent(ctx, "e1")
	if len(d.Responses) != 2 || d.Responses[0].User.ID != alan.ID {
		t.Fatalf("responses: %+v", d.Responses)
	}
	if v := d.Responses[0].Votes; len(v) != 1 || v["e1-b"] != model.VoteYes {
		t.Fatalf("votes: %+v", v)
	}
	if err := s.SetResponse(ctx, "e1", alan.ID, map[string]model.Vote{"nope": model.VoteYes}, t0); err == nil {
		t.Fatal("expected error for unknown slot")
	}

	// Update: keep b (moved), drop a, add c; final slot a gets cleared.
	must(t, s.SetClosed(ctx, "e1", true, "e1-a", t0))
	e := d.Event
	e.Title = "Dinner"
	e.UpdatedAt = t0.Add(time.Hour)
	e.Slots = []model.Slot{slot("e1-b", 5), slot("e1-c", 6)}
	must(t, s.UpdateEvent(ctx, e))
	d, _ = s.GetEvent(ctx, "e1")
	if d.Title != "Dinner" || len(d.Slots) != 2 || d.Slots[0].ID != "e1-b" || !d.Slots[0].Start.Equal(t0.Add(5*time.Hour)) {
		t.Fatalf("update: %+v", d)
	}
	if d.FinalSlotID != "" || !d.Closed {
		t.Fatalf("final slot should be cleared: %+v", d)
	}
	if v := d.Responses[0].Votes; v["e1-b"] != model.VoteYes {
		t.Fatalf("votes for kept slot lost: %+v", v)
	}

	must(t, s.SetClosed(ctx, "e1", true, "e1-c", t0))
	d, _ = s.GetEvent(ctx, "e1")
	if d.FinalSlotID != "e1-c" {
		t.Fatalf("final: %q", d.FinalSlotID)
	}
	must(t, s.SetClosed(ctx, "e1", false, "", t0))

	// Withdraw.
	must(t, s.DeleteResponse(ctx, "e1", alan.ID))
	d, _ = s.GetEvent(ctx, "e1")
	if len(d.Responses) != 1 {
		t.Fatalf("responses after delete: %+v", d.Responses)
	}

	must(t, s.DeleteEvent(ctx, "e1"))
	if _, err := s.GetEvent(ctx, "e1"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
}

func TestNotFound(t *testing.T) {
	s := open(t)
	e := event("missing")
	for name, err := range map[string]error{
		"update": s.UpdateEvent(ctx, e),
		"close":  s.SetClosed(ctx, "missing", true, "", t0),
		"delete": s.DeleteEvent(ctx, "missing"),
	} {
		if !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestCreateEventDuplicateSlot(t *testing.T) {
	s := open(t)
	e := event("e1")
	e.Slots = append(e.Slots, e.Slots[0])
	if err := s.CreateEvent(ctx, e); err == nil {
		t.Fatal("expected error")
	}
	if _, err := s.GetEvent(ctx, "e1"); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("transaction should have been rolled back")
	}
	must(t, s.CreateEvent(ctx, event("e2")))
	e2 := event("e2")
	e2.Slots = []model.Slot{slot("e2-new", 1), slot("e2-new", 2)}
	if err := s.UpdateEvent(ctx, e2); err == nil {
		t.Fatal("expected error")
	}
}

func TestListEvents(t *testing.T) {
	s := open(t)
	must(t, s.CreateEvent(ctx, event("e1")))
	e2 := event("e2")
	e2.Owner = alan
	e2.UpdatedAt = t0.Add(time.Hour)
	must(t, s.CreateEvent(ctx, e2))
	e3 := event("e3")
	e3.Owner = alan
	must(t, s.CreateEvent(ctx, e3))
	must(t, s.SetResponse(ctx, "e2", ada.ID, map[string]model.Vote{"e2-a": model.VoteYes}, t0))

	list, err := s.ListEvents(ctx, ada.ID)
	must(t, err)
	if len(list) != 2 || list[0].ID != "e2" || list[1].ID != "e1" {
		t.Fatalf("list: %+v", list)
	}
	if list[0].Role != model.RoleParticipant || list[1].Role != model.RoleOwner {
		t.Fatalf("roles: %+v", list)
	}
	if list[0].SlotCount != 2 || list[0].ResponseCount != 1 || !list[0].FirstStart.Equal(t0.Add(time.Hour)) || !list[0].LastEnd.Equal(t0.Add(3*time.Hour)) {
		t.Fatalf("summary: %+v", list[0])
	}

	empty, err := s.ListEvents(ctx, "nobody")
	must(t, err)
	if len(empty) != 0 {
		t.Fatalf("expected empty list, got %+v", empty)
	}
}

func TestListEventsWithoutSlots(t *testing.T) {
	s := open(t)
	e := event("e1")
	e.Slots = nil
	must(t, s.CreateEvent(ctx, e))
	list, err := s.ListEvents(ctx, ada.ID)
	must(t, err)
	if list[0].FirstStart != nil || list[0].LastEnd != nil {
		t.Fatalf("expected nil bounds: %+v", list[0])
	}
}

func TestClosedDatabase(t *testing.T) {
	s := open(t)
	must(t, s.CreateEvent(ctx, event("e1")))
	mustClose(t, s)
	errs := []error{
		s.UpsertUser(ctx, ada),
		s.CreateSession(ctx, "h", ada.ID, t0),
		s.DeleteSession(ctx, "h"),
		s.CreateEvent(ctx, event("e2")),
		s.UpdateEvent(ctx, event("e1")),
		s.SetClosed(ctx, "e1", true, "", t0),
		s.DeleteEvent(ctx, "e1"),
		s.SetResponse(ctx, "e1", ada.ID, nil, t0),
		s.DeleteResponse(ctx, "e1", ada.ID),
	}
	if _, err := s.SessionUser(ctx, "h", t0); err == nil || errors.Is(err, model.ErrNotFound) {
		t.Errorf("SessionUser: %v", err)
	}
	if _, err := s.GetEvent(ctx, "e1"); err == nil || errors.Is(err, model.ErrNotFound) {
		t.Errorf("GetEvent: %v", err)
	}
	if _, err := s.ListEvents(ctx, ada.ID); err == nil {
		t.Error("ListEvents: expected error")
	}
	for i, err := range errs {
		if err == nil {
			t.Errorf("call %d: expected error", i)
		}
	}
}

// Dropping tables lets us exercise errors that happen midway through
// multi-statement operations.
func TestPartialFailures(t *testing.T) {
	t.Run("votes table missing", func(t *testing.T) {
		s := open(t)
		must(t, s.CreateEvent(ctx, event("e1")))
		must(t, s.SetResponse(ctx, "e1", ada.ID, nil, t0))
		_, err := s.db.ExecContext(ctx, `DROP TABLE votes`)
		must(t, err)
		if _, err := s.GetEvent(ctx, "e1"); err == nil {
			t.Error("GetEvent: expected error")
		}
		if err := s.SetResponse(ctx, "e1", ada.ID, nil, t0); err == nil {
			t.Error("SetResponse: expected error")
		}
		if err := s.DeleteResponse(ctx, "e1", ada.ID); err == nil {
			t.Error("DeleteResponse: expected error")
		}
	})
	t.Run("responses table missing", func(t *testing.T) {
		s := open(t)
		must(t, s.CreateEvent(ctx, event("e1")))
		_, err := s.db.ExecContext(ctx, `DROP TABLE votes; DROP TABLE responses`)
		must(t, err)
		if _, err := s.GetEvent(ctx, "e1"); err == nil {
			t.Error("GetEvent: expected error")
		}
	})
	t.Run("slots table missing", func(t *testing.T) {
		s := open(t)
		must(t, s.CreateEvent(ctx, event("e1")))
		_, err := s.db.ExecContext(ctx, `DROP TABLE votes; DROP TABLE slots`)
		must(t, err)
		if _, err := s.GetEvent(ctx, "e1"); err == nil {
			t.Error("GetEvent: expected error")
		}
		if err := s.UpdateEvent(ctx, event("e1")); err == nil {
			t.Error("UpdateEvent: expected error")
		}
		e := event("e2")
		e.Slots = nil
		if err := s.CreateEvent(ctx, e); err != nil {
			t.Errorf("CreateEvent without slots: %v", err)
		}
	})
	t.Run("upsert response conflict", func(t *testing.T) {
		s := open(t)
		must(t, s.CreateEvent(ctx, event("e1")))
		if err := s.SetResponse(ctx, "e1", "nobody", nil, t0); err == nil {
			t.Error("expected foreign key error")
		}
	})
}

func TestMigrateError(t *testing.T) {
	s := open(t)
	mustClose(t, s)
	if err := s.migrate(ctx); err == nil {
		t.Fatal("expected error")
	}
}
