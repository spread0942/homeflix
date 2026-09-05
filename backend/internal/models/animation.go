package models

import (
	"time"

	"github.com/google/uuid"
)

type Animation struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	VideoPath   string    `json:"-"`
	PosterPath  string    `json:"-"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	PosterURL   string    `json:"poster_url"`
	StreamURL   string    `json:"stream_url"`
}

func (a *Animation) WithURLs() {
	id := a.ID.String()
	a.PosterURL = "/api/animations/" + id + "/poster"
	a.StreamURL = "/api/animations/" + id + "/stream"
}
