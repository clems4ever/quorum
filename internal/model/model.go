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
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}
