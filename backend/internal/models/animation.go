package models

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Series struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	PosterPath  *string     `json:"-"`
	Kind        string      `json:"kind"`
	CreatedAt   time.Time   `json:"created_at"`
	PosterURL   string      `json:"poster_url"`
	EntryCount  int         `json:"entry_count"`
	Entries     []Animation `json:"entries,omitempty"`
}

func (s *Series) WithPosterURL() {
	if s.PosterPath != nil && *s.PosterPath != "" {
		s.PosterURL = "/api/series/" + s.ID.String() + "/poster"
	}
}

type Animation struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	VideoPath      string     `json:"-"`
	PosterPath     string     `json:"-"`
	ContentType    string     `json:"content_type"`
	PlaybackStatus string     `json:"playback_status"`
	SeriesID       *uuid.UUID `json:"series_id,omitempty"`
	SeriesName     string     `json:"series_name,omitempty"`
	Season         *int       `json:"season,omitempty"`
	Episode        *int       `json:"episode,omitempty"`
	SortOrder      int        `json:"sort_order"`
	CreatedAt      time.Time  `json:"created_at"`
	PosterURL      string     `json:"poster_url"`
	StreamURL      string     `json:"stream_url"`
}

func (a *Animation) WithURLs() {
	id := a.ID.String()
	a.PosterURL = "/api/animations/" + id + "/poster"
	a.StreamURL = "/api/animations/" + id + "/stream"
}

// LibraryItem is a home-grid card: either a series or a standalone film.
type LibraryItem struct {
	Type        string     `json:"type"` // "series" | "film"
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	PosterURL   string     `json:"poster_url"`
	EntryCount  int        `json:"entry_count,omitempty"`
	Kind        string     `json:"kind,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	FilmID      *uuid.UUID `json:"film_id,omitempty"` // for type=film, same as ID
}

// WatchProgress is the saved playback position for one film/episode.
type WatchProgress struct {
	AnimationID     uuid.UUID `json:"animation_id"`
	PositionSeconds float64   `json:"position_seconds"`
	DurationSeconds float64   `json:"duration_seconds"`
	Completed       bool      `json:"completed"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ContinueWatchingItem is a home-row card for resume / series complete.
type ContinueWatchingItem struct {
	AnimationID     uuid.UUID  `json:"animation_id"`
	Name            string     `json:"name"`
	PosterURL       string     `json:"poster_url"`
	PositionSeconds float64    `json:"position_seconds"`
	DurationSeconds float64    `json:"duration_seconds"`
	Completed       bool       `json:"completed"`
	UpdatedAt       time.Time  `json:"updated_at"`
	SeriesID        *uuid.UUID `json:"series_id,omitempty"`
	SeriesName      string     `json:"series_name,omitempty"`
	Season          *int       `json:"season,omitempty"`
	Episode         *int       `json:"episode,omitempty"`
	EntryLabel      string     `json:"entry_label,omitempty"`
	SeriesComplete  bool       `json:"series_complete"`
}

// ProgressNearEnd reports whether position counts as finished.
func ProgressNearEnd(position, duration float64) bool {
	if duration <= 0 {
		return false
	}
	cutoff := duration * 0.9
	if alt := duration - 30; alt > cutoff {
		cutoff = alt
	}
	return position >= cutoff
}

// EntryLabel builds an SxEy / Part N label matching the frontend helper.
func EntryLabel(season, episode *int, sortOrder int) string {
	var bits []string
	if season != nil {
		bits = append(bits, "S"+strconv.Itoa(*season))
	}
	if episode != nil {
		bits = append(bits, "E"+strconv.Itoa(*episode))
	}
	if len(bits) == 0 && sortOrder > 0 {
		return "Part " + strconv.Itoa(sortOrder)
	}
	if len(bits) == 0 {
		return ""
	}
	out := bits[0]
	for i := 1; i < len(bits); i++ {
		out += " · " + bits[i]
	}
	return out
}
