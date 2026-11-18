// Package model holds the domain types shared across the application.
package model

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// User is an authenticated person.
type User struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Mode describes the granularity of an event's slots.
type Mode string

const (
	// ModeDays means every slot covers whole days.
	ModeDays Mode = "days"
	// ModeHours means every slot is a time range within a day.
	ModeHours Mode = "hours"
)

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool { return m == ModeDays || m == ModeHours }

// Vote is a participant's answer for one slot. Absence of a vote means "no".
type Vote string

const (
	// VoteYes means the participant is available.
	VoteYes Vote = "yes"
	// VoteMaybe means the participant could make it if needed.
	VoteMaybe Vote = "maybe"
)

// Valid reports whether v is a known vote value.
func (v Vote) Valid() bool { return v == VoteYes || v == VoteMaybe }

// Slot is a candidate time range for an event.
type Slot struct {
	ID    string    `json:"id"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Event is a poll proposed by its owner.
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Mode        Mode      `json:"mode"`
	Owner       User      `json:"owner"`
	Slots       []Slot    `json:"slots"`
	Closed      bool      `json:"closed"`
	FinalSlotID string    `json:"finalSlotId,omitempty"`
	Cover       *Cover    `json:"cover,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Image is the cover image of an event.
type Image struct {
	ContentType string
	Data        []byte
	// Version changes whenever the image does (a content hash).
	Version string
	Focus   Focus
}

// Focus is the point of an image that stays visible when it is cropped, as
// fractions of its width and height (0.5, 0.5 is the center).
type Focus struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CenterFocus is the default focus.
var CenterFocus = Focus{X: 0.5, Y: 0.5}

// Valid reports whether both coordinates are within [0, 1].
func (f Focus) Valid() bool { return f.X >= 0 && f.X <= 1 && f.Y >= 0 && f.Y <= 1 }

// Cover describes the cover image of an event.
type Cover struct {
	// Version changes whenever the image does; it is not exposed as is.
	Version string `json:"-"`
	// URL is filled in by the API layer.
	URL   string `json:"url"`
	Focus Focus  `json:"focus"`
}

// Response is the set of votes a participant gave to an event.
type Response struct {
	User      User            `json:"user"`
	Votes     map[string]Vote `json:"votes"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// EventDetail is an event together with every participant's response.
type EventDetail struct {
	Event
	Responses []Response `json:"responses"`
}

// Role is the relationship between a user and an event.
type Role string

const (
	// RoleOwner means the user created the event.
	RoleOwner Role = "owner"
	// RoleParticipant means the user voted on the event.
	RoleParticipant Role = "participant"
)

// EventSummary is a compact view of an event used in listings.
type EventSummary struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Location      string     `json:"location"`
	Mode          Mode       `json:"mode"`
	Owner         User       `json:"owner"`
	Role          Role       `json:"role"`
	Closed        bool       `json:"closed"`
	SlotCount     int        `json:"slotCount"`
	ResponseCount int        `json:"responseCount"`
	FirstStart    *time.Time `json:"firstStart,omitempty"`
	LastEnd       *time.Time `json:"lastEnd,omitempty"`
	Cover         *Cover     `json:"cover,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}
