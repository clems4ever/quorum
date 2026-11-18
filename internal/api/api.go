// Package api exposes the JSON HTTP API used by the web client.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"quorum/internal/model"
)

// Limits applied to user input.
const (
	MaxTitle       = 120
	MaxDescription = 2000
	MaxLocation    = 200
	MaxSlots       = 300
	maxBody        = 1 << 20
)

// Store is the persistence used by the API.
type Store interface {
	CreateEvent(ctx context.Context, e model.Event) error
	UpdateEvent(ctx context.Context, e model.Event) error
	SetClosed(ctx context.Context, eventID string, closed bool, finalSlotID string, now time.Time) error
	DeleteEvent(ctx context.Context, eventID string) error
	GetEvent(ctx context.Context, eventID string) (model.EventDetail, error)
	ListEvents(ctx context.Context, userID string) ([]model.EventSummary, error)
	SetResponse(ctx context.Context, eventID, userID string, votes map[string]model.Vote, now time.Time) error
	DeleteResponse(ctx context.Context, eventID, userID string) error
}

// Authenticator resolves the user of a request.
type Authenticator interface {
	User(r *http.Request) (model.User, error)
}

// API serves /api/* routes.
type API struct {
	Store  Store
	Auth   Authenticator
	Now    func() time.Time
	NewID  func() (string, error)
	Logger *slog.Logger
}

// Register mounts the routes on mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/me", a.authed(a.me))
	mux.HandleFunc("GET /api/events", a.authed(a.listEvents))
	mux.HandleFunc("POST /api/events", a.authed(a.createEvent))
	mux.HandleFunc("GET /api/events/{id}", a.authed(a.getEvent))
	mux.HandleFunc("PUT /api/events/{id}", a.authed(a.updateEvent))
	mux.HandleFunc("DELETE /api/events/{id}", a.authed(a.deleteEvent))
	mux.HandleFunc("POST /api/events/{id}/close", a.authed(a.closeEvent))
	mux.HandleFunc("POST /api/events/{id}/reopen", a.authed(a.reopenEvent))
	mux.HandleFunc("PUT /api/events/{id}/response", a.authed(a.setResponse))
	mux.HandleFunc("DELETE /api/events/{id}/response", a.authed(a.deleteResponse))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, errRouteNotFound)
	})
}

func (a *API) authed(next func(http.ResponseWriter, *http.Request, model.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := a.Auth.User(r)
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, errNotSignedIn)
			return
		}
		if err != nil {
			a.internal(w, err)
			return
		}
		next(w, r, u)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// An encoding error here means the client went away; nothing to do.
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is an error reported to clients. Code is stable and meant for
// translation; Message is a human readable English description.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

var (
	errRouteNotFound      = apiError{"not_found", "not found"}
	errNotSignedIn        = apiError{"not_signed_in", "not signed in"}
	errInternal           = apiError{"internal", "internal error"}
	errInvalidJSON        = apiError{"invalid_json", "invalid JSON body"}
	errInvalidMode        = apiError{"invalid_mode", "mode must be \"days\" or \"hours\""}
	errEventNotFound      = apiError{"event_not_found", "event not found"}
	errNotOrganizer       = apiError{"not_organizer", "only the organizer can do this"}
	errModeImmutable      = apiError{"mode_immutable", "the mode of an event cannot be changed"}
	errUnknownSlot        = apiError{"unknown_slot", "unknown slot"}
	errPollClosed         = apiError{"poll_closed", "this poll is closed"}
	errInvalidVote        = apiError{"invalid_vote", "votes must be \"yes\", \"maybe\" or \"no\""}
	errTitleRequired      = apiError{"title_required", "title is required"}
	errTitleTooLong       = apiError{"title_too_long", "title is too long"}
	errDescriptionTooLong = apiError{"description_too_long", "description is too long"}
	errLocationTooLong    = apiError{"location_too_long", "location is too long"}
	errNoSlots            = apiError{"no_slots", "at least one slot is required"}
	errTooManySlots       = apiError{"too_many_slots", "too many slots"}
	errInvalidSlot        = apiError{"invalid_slot", "every slot must end after it starts"}
	errDuplicateSlot      = apiError{"duplicate_slot", "slots must be unique"}
)

func writeError(w http.ResponseWriter, status int, e apiError) {
	writeJSON(w, status, e)
}

func (a *API) internal(w http.ResponseWriter, err error) {
	a.Logger.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, errInternal)
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (a *API) me(w http.ResponseWriter, _ *http.Request, u model.User) {
	writeJSON(w, http.StatusOK, u)
}

func (a *API) listEvents(w http.ResponseWriter, r *http.Request, u model.User) {
	list, err := a.Store.ListEvents(r.Context(), u.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type slotInput struct {
	ID    string    `json:"id,omitempty"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type eventInput struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Location    string      `json:"location"`
	Mode        model.Mode  `json:"mode"`
	Slots       []slotInput `json:"slots"`
}

func (in *eventInput) validate() *apiError {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Location = strings.TrimSpace(in.Location)
	switch {
	case in.Title == "":
		return &errTitleRequired
	case len(in.Title) > MaxTitle:
		return &errTitleTooLong
	case len(in.Description) > MaxDescription:
		return &errDescriptionTooLong
	case len(in.Location) > MaxLocation:
		return &errLocationTooLong
	case len(in.Slots) == 0:
		return &errNoSlots
	case len(in.Slots) > MaxSlots:
		return &errTooManySlots
	}
	seen := map[[2]int64]bool{}
	for _, s := range in.Slots {
		if s.Start.IsZero() || !s.End.After(s.Start) {
			return &errInvalidSlot
		}
		key := [2]int64{s.Start.Unix(), s.End.Unix()}
		if seen[key] {
			return &errDuplicateSlot
		}
		seen[key] = true
	}
	return nil
}

// buildSlots assigns IDs to new slots. Existing IDs are kept only when they
// belong to the event (known); unknown IDs are treated as new slots.
func (a *API) buildSlots(in []slotInput, known map[string]bool) ([]model.Slot, error) {
	slots := make([]model.Slot, 0, len(in))
	for _, s := range in {
		id := s.ID
		if !known[id] {
			var err error
			if id, err = a.NewID(); err != nil {
				return nil, err
			}
		}
		known[id] = false // an ID may only be used once
		slots = append(slots, model.Slot{ID: id, Start: s.Start.UTC(), End: s.End.UTC()})
	}
	return slots, nil
}

func (a *API) createEvent(w http.ResponseWriter, r *http.Request, u model.User) {
	var in eventInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}
	if e := in.validate(); e != nil {
		writeError(w, http.StatusBadRequest, *e)
		return
	}
	if !in.Mode.Valid() {
		writeError(w, http.StatusBadRequest, errInvalidMode)
		return
	}
	id, err := a.NewID()
	if err != nil {
		a.internal(w, err)
		return
	}
	slots, err := a.buildSlots(in.Slots, map[string]bool{})
	if err != nil {
		a.internal(w, err)
		return
	}
	now := a.Now().UTC()
	e := model.Event{
		ID: id, Title: in.Title, Description: in.Description, Location: in.Location,
		Mode: in.Mode, Owner: u, Slots: slots, CreatedAt: now, UpdatedAt: now,
	}
	if err := a.Store.CreateEvent(r.Context(), e); err != nil {
		a.internal(w, err)
		return
	}
	a.respondEvent(w, r, id, http.StatusCreated)
}

func (a *API) respondEvent(w http.ResponseWriter, r *http.Request, id string, status int) {
	d, err := a.Store.GetEvent(r.Context(), id)
	if errors.Is(err, model.ErrNotFound) {
		writeError(w, http.StatusNotFound, errEventNotFound)
		return
	}
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, status, d)
}

func (a *API) getEvent(w http.ResponseWriter, r *http.Request, _ model.User) {
	a.respondEvent(w, r, r.PathValue("id"), http.StatusOK)
}

// loadOwned loads the event and checks that u owns it. It writes the error
// response and returns ok=false on failure.
func (a *API) loadOwned(w http.ResponseWriter, r *http.Request, u model.User) (model.EventDetail, bool) {
	d, ok := a.load(w, r)
	if ok && d.Owner.ID != u.ID {
		writeError(w, http.StatusForbidden, errNotOrganizer)
		return d, false
	}
	return d, ok
}

func (a *API) load(w http.ResponseWriter, r *http.Request) (model.EventDetail, bool) {
	d, err := a.Store.GetEvent(r.Context(), r.PathValue("id"))
	if errors.Is(err, model.ErrNotFound) {
		writeError(w, http.StatusNotFound, errEventNotFound)
		return d, false
	}
	if err != nil {
		a.internal(w, err)
		return d, false
	}
	return d, true
}

func (a *API) updateEvent(w http.ResponseWriter, r *http.Request, u model.User) {
	var in eventInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}
	if e := in.validate(); e != nil {
		writeError(w, http.StatusBadRequest, *e)
		return
	}
	d, ok := a.loadOwned(w, r, u)
	if !ok {
		return
	}
	if in.Mode != "" && in.Mode != d.Mode {
		writeError(w, http.StatusBadRequest, errModeImmutable)
		return
	}
	known := map[string]bool{}
	for _, s := range d.Slots {
		known[s.ID] = true
	}
	slots, err := a.buildSlots(in.Slots, known)
	if err != nil {
		a.internal(w, err)
		return
	}
	e := d.Event
	e.Title, e.Description, e.Location, e.Slots = in.Title, in.Description, in.Location, slots
	e.UpdatedAt = a.Now().UTC()
	if err := a.Store.UpdateEvent(r.Context(), e); err != nil {
		a.internal(w, err)
		return
	}
	a.respondEvent(w, r, e.ID, http.StatusOK)
}

func (a *API) deleteEvent(w http.ResponseWriter, r *http.Request, u model.User) {
	d, ok := a.loadOwned(w, r, u)
	if !ok {
		return
	}
	if err := a.Store.DeleteEvent(r.Context(), d.ID); err != nil {
		a.internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type closeInput struct {
	FinalSlotID string `json:"finalSlotId"`
}

func (a *API) closeEvent(w http.ResponseWriter, r *http.Request, u model.User) {
	var in closeInput
	if r.ContentLength != 0 {
		if err := decode(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, errInvalidJSON)
			return
		}
	}
	d, ok := a.loadOwned(w, r, u)
	if !ok {
		return
	}
	if in.FinalSlotID != "" && !hasSlot(d.Slots, in.FinalSlotID) {
		writeError(w, http.StatusBadRequest, errUnknownSlot)
		return
	}
	a.setClosed(w, r, d.ID, true, in.FinalSlotID)
}

func (a *API) reopenEvent(w http.ResponseWriter, r *http.Request, u model.User) {
	d, ok := a.loadOwned(w, r, u)
	if !ok {
		return
	}
	a.setClosed(w, r, d.ID, false, "")
}

func (a *API) setClosed(w http.ResponseWriter, r *http.Request, id string, closed bool, final string) {
	if err := a.Store.SetClosed(r.Context(), id, closed, final, a.Now().UTC()); err != nil {
		a.internal(w, err)
		return
	}
	a.respondEvent(w, r, id, http.StatusOK)
}

func hasSlot(slots []model.Slot, id string) bool {
	for _, s := range slots {
		if s.ID == id {
			return true
		}
	}
	return false
}

type responseInput struct {
	Votes map[string]model.Vote `json:"votes"`
}

func (a *API) setResponse(w http.ResponseWriter, r *http.Request, u model.User) {
	var in responseInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}
	d, ok := a.load(w, r)
	if !ok {
		return
	}
	if d.Closed {
		writeError(w, http.StatusConflict, errPollClosed)
		return
	}
	votes := map[string]model.Vote{}
	for id, v := range in.Votes {
		if !hasSlot(d.Slots, id) {
			writeError(w, http.StatusBadRequest, errUnknownSlot)
			return
		}
		if v == "" || v == "no" {
			continue
		}
		if !v.Valid() {
			writeError(w, http.StatusBadRequest, errInvalidVote)
			return
		}
		votes[id] = v
	}
	if err := a.Store.SetResponse(r.Context(), d.ID, u.ID, votes, a.Now().UTC()); err != nil {
		a.internal(w, err)
		return
	}
	a.respondEvent(w, r, d.ID, http.StatusOK)
}

func (a *API) deleteResponse(w http.ResponseWriter, r *http.Request, u model.User) {
	d, ok := a.load(w, r)
	if !ok {
		return
	}
	if d.Closed {
		writeError(w, http.StatusConflict, errPollClosed)
		return
	}
	if err := a.Store.DeleteResponse(r.Context(), d.ID, u.ID); err != nil {
		a.internal(w, err)
		return
	}
	a.respondEvent(w, r, d.ID, http.StatusOK)
}
