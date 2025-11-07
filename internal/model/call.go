package model

import "time"

// Destination represents a destination to send a call to.
type Destination struct {
	Type string   `json:"type" yaml:"type"`
	To   []string `json:"to,omitempty" yaml:"to,omitempty"`
}

// Trigger represents a scheduling mechanism for a call.
type Trigger struct {
	ScheduledAt time.Time `json:"scheduled_at,omitempty" yaml:"scheduled_at,omitempty"`
	Cron        string    `json:"cron,omitempty" yaml:"cron,omitempty"`
	RRule       string    `json:"rrule,omitempty" yaml:"rrule,omitempty"`
	DStart      string    `json:"dstart,omitempty" yaml:"dstart,omitempty"`
	Delta       string    `json:"delta,omitempty" yaml:"delta,omitempty"`
	Sequence    string    `json:"sequence,omitempty" yaml:"sequence,omitempty"`
}

// Call represents a message to be sent to a destination.
type Call struct {
	ID           string        `json:"id" yaml:"id"`
	Author       string        `json:"author,omitempty" yaml:"author,omitempty"`
	Subject      string        `json:"subject,omitempty" yaml:"subject,omitempty"`
	Content      string                 `json:"content" yaml:"content"`
	Destinations []Destination          `json:"destinations" yaml:"destinations"`
	Triggers     []Trigger              `json:"triggers" yaml:"triggers"`
	Data         map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`

	Campaign Campaign `json:"campaign,omitempty" yaml:"campaign,omitempty"`

	// Fields for expanded calls, not to be set in YAML
	ScheduledAt time.Time `json:"-" yaml:"-"`
}

// Event represents an event invocation.
type Event struct {
	Destinations []Destination `json:"destinations,omitempty" yaml:"destinations,omitempty"`
	Sequence     string        `json:"sequence" yaml:"sequence"`
	StartTime    time.Time     `json:"start_time" yaml:"start_time"`
}

// Campaign represents a campaign.
type Campaign struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	IconURL string `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`
}
