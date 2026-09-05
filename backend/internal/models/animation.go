package models

import (
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
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	VideoPath   string     `json:"-"`
	PosterPath  string     `json:"-"`
	ContentType string     `json:"content_type"`
	SeriesID    *uuid.UUID `json:"series_id,omitempty"`
	SeriesName  string     `json:"series_name,omitempty"`
	Season      *int       `json:"season,omitempty"`
	Episode     *int       `json:"episode,omitempty"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	PosterURL   string     `json:"poster_url"`
	StreamURL   string     `json:"stream_url"`
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
