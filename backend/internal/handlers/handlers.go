package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/spread/homeflix/internal/imageconv"
	"github.com/spread/homeflix/internal/models"
	"github.com/spread/homeflix/internal/store"
	"github.com/spread/homeflix/internal/transcode"
)

const maxUpload = 4 << 30 // 4 GiB

type API struct {
	Store     *store.Store
	MediaRoot string

	transcoding sync.Map // uuid.UUID string -> struct{}
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/library", a.Library)

	r.Get("/series", a.ListSeries)
	r.Post("/series", a.CreateSeries)
	r.Get("/series/{id}", a.GetSeries)
	r.Put("/series/{id}", a.UpdateSeries)
	r.Delete("/series/{id}", a.DeleteSeries)
	r.Get("/series/{id}/poster", a.ServeSeriesPoster)

	r.Get("/animations", a.ListAnimations)
	r.Post("/animations", a.CreateAnimation)
	r.Get("/animations/{id}", a.GetAnimation)
	r.Put("/animations/{id}", a.UpdateAnimation)
	r.Delete("/animations/{id}", a.DeleteAnimation)
	r.Post("/animations/{id}/transcode", a.TranscodeAnimation)
	r.Get("/animations/{id}/poster", a.ServePoster)
	r.Get("/animations/{id}/stream", a.StreamVideo)

	r.Get("/continue-watching", a.ContinueWatching)
	r.Get("/progress/{id}", a.GetProgress)
	r.Put("/progress/{id}", a.PutProgress)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return r
}

func (a *API) Library(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Library(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load library")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) ContinueWatching(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.ListContinueWatching(r.Context(), 12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load continue watching")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) GetProgress(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := a.Store.GetProgress(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get progress")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type putProgressBody struct {
	PositionSeconds float64 `json:"position_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
	Completed       *bool   `json:"completed"`
}

func (a *API) PutProgress(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := a.Store.Get(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load animation")
		return
	}

	var body putProgressBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.PositionSeconds < 0 {
		body.PositionSeconds = 0
	}
	if body.DurationSeconds < 0 {
		body.DurationSeconds = 0
	}

	completed := false
	if body.Completed != nil {
		completed = *body.Completed
	}
	if !completed && models.ProgressNearEnd(body.PositionSeconds, body.DurationSeconds) {
		completed = true
	}

	p := &models.WatchProgress{
		AnimationID:     id,
		PositionSeconds: body.PositionSeconds,
		DurationSeconds: body.DurationSeconds,
		Completed:       completed,
	}
	if err := a.Store.UpsertProgress(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save progress")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) ListSeries(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.ListSeries(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list series")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) GetSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.GetSeries(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get series")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) CreateSeries(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// also allow JSON/simple form without files
		if err2 := r.ParseForm(); err2 != nil {
			writeError(w, http.StatusBadRequest, "invalid form")
			return
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if kind == "" {
		kind = "franchise"
	}
	switch kind {
	case "franchise", "tv", "anime":
	default:
		writeError(w, http.StatusBadRequest, "kind must be franchise, tv, or anime")
		return
	}

	id := uuid.New()
	var posterRel *string
	posterFile, _, err := r.FormFile("poster")
	if err == nil {
		defer posterFile.Close()
		rel := filepath.Join("series_posters", id.String()+".webp")
		abs := filepath.Join(a.MediaRoot, rel)
		if err := saveImageAsWebP(posterFile, abs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save poster")
			return
		}
		posterRel = &rel
	}

	ser := &models.Series{
		ID:          id,
		Name:        name,
		Description: description,
		PosterPath:  posterRel,
		Kind:        kind,
	}
	if err := a.Store.CreateSeries(r.Context(), ser); err != nil {
		if posterRel != nil {
			_ = os.Remove(filepath.Join(a.MediaRoot, *posterRel))
		}
		writeError(w, http.StatusInternalServerError, "failed to save series")
		return
	}
	ser.WithPosterURL()
	ser.EntryCount = 0
	ser.Entries = []models.Animation{}
	writeJSON(w, http.StatusCreated, ser)
}

func (a *API) UpdateSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.GetSeries(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get series")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			writeError(w, http.StatusBadRequest, "invalid form")
			return
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if kind == "" {
		kind = existing.Kind
	}
	switch kind {
	case "franchise", "tv", "anime":
	default:
		writeError(w, http.StatusBadRequest, "kind must be franchise, tv, or anime")
		return
	}

	posterRel := existing.PosterPath
	var newPosterAbs string
	posterFile, _, err := r.FormFile("poster")
	if err == nil {
		defer posterFile.Close()
		rel := filepath.Join("series_posters", id.String()+".webp")
		abs := filepath.Join(a.MediaRoot, rel)
		if err := saveImageAsWebP(posterFile, abs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save poster")
			return
		}
		posterRel = &rel
		newPosterAbs = abs
	}

	existing.Name = name
	existing.Description = description
	existing.Kind = kind
	existing.PosterPath = posterRel
	if err := a.Store.UpdateSeries(r.Context(), existing); err != nil {
		if newPosterAbs != "" {
			_ = os.Remove(newPosterAbs)
		}
		writeError(w, http.StatusInternalServerError, "failed to update series")
		return
	}

	updated, err := a.Store.GetSeries(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload series")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) DeleteSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ser, entries, err := a.Store.DeleteSeries(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete series")
		return
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(a.MediaRoot, e.VideoPath))
		_ = os.Remove(filepath.Join(a.MediaRoot, e.PosterPath))
	}
	if ser.PosterPath != nil && *ser.PosterPath != "" {
		_ = os.Remove(filepath.Join(a.MediaRoot, *ser.PosterPath))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ServeSeriesPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rel, err := a.Store.SeriesPosterFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get poster")
		return
	}
	path := filepath.Join(a.MediaRoot, rel)
	if strings.HasSuffix(strings.ToLower(path), ".webp") {
		w.Header().Set("Content-Type", "image/webp")
	}
	http.ServeFile(w, r, path)
}

func (a *API) ListAnimations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	items, err := a.Store.List(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list animations")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) GetAnimation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) CreateAnimation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var seriesID *uuid.UUID
	if raw := strings.TrimSpace(r.FormValue("series_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid series_id")
			return
		}
		if _, err := a.Store.GetSeries(r.Context(), id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "series not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to validate series")
			return
		}
		seriesID = &id
	}

	season, err := parseOptionalInt(r.FormValue("season"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid season")
		return
	}
	episode, err := parseOptionalInt(r.FormValue("episode"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode")
		return
	}
	sortOrder := 0
	if raw := strings.TrimSpace(r.FormValue("sort_order")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sort_order")
			return
		}
		sortOrder = n
	}

	videoFile, videoHeader, err := r.FormFile("video")
	if err != nil {
		writeError(w, http.StatusBadRequest, "video file is required")
		return
	}
	defer videoFile.Close()

	posterFile, _, posterErr := r.FormFile("poster")
	hasPoster := posterErr == nil
	if hasPoster {
		defer posterFile.Close()
	}

	id := uuid.New()
	videoExt := extOr(videoHeader.Filename, ".mp4")
	videoRel := filepath.Join("videos", id.String()+videoExt)
	posterRel := filepath.Join("posters", id.String()+".webp")
	videoAbs := filepath.Join(a.MediaRoot, videoRel)
	posterAbs := filepath.Join(a.MediaRoot, posterRel)

	if err := os.MkdirAll(filepath.Dir(videoAbs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare storage")
		return
	}
	if err := os.MkdirAll(filepath.Dir(posterAbs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare storage")
		return
	}

	if err := saveUpload(videoFile, videoAbs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save video")
		return
	}
	if hasPoster {
		if err := saveImageAsWebP(posterFile, posterAbs); err != nil {
			_ = os.Remove(videoAbs)
			writeError(w, http.StatusInternalServerError, "failed to save poster")
			return
		}
	} else if err := imageconv.ExtractFrameAsWebP(videoAbs, posterAbs); err != nil {
		_ = os.Remove(videoAbs)
		log.Printf("auto poster for %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to generate poster from video")
		return
	}

	contentType := videoHeader.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeFromExt(videoExt)
	}

	status := "ready"
	if transcode.NeedsTranscode(videoExt, contentType) {
		status = "processing"
	}

	anim := &models.Animation{
		ID:             id,
		Name:           name,
		Description:    description,
		VideoPath:      videoRel,
		PosterPath:     posterRel,
		ContentType:    contentType,
		PlaybackStatus: status,
		SeriesID:       seriesID,
		Season:         season,
		Episode:        episode,
		SortOrder:      sortOrder,
	}
	if err := a.Store.Create(r.Context(), anim); err != nil {
		_ = os.Remove(videoAbs)
		_ = os.Remove(posterAbs)
		writeError(w, http.StatusInternalServerError, "failed to save animation")
		return
	}
	if status == "processing" {
		a.StartTranscode(id)
	}
	// reload for series_name
	if full, err := a.Store.Get(r.Context(), anim.ID); err == nil {
		anim = full
	} else {
		anim.WithURLs()
	}
	writeJSON(w, http.StatusCreated, anim)
}

func (a *API) UpdateAnimation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			writeError(w, http.StatusBadRequest, "invalid form")
			return
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var seriesID *uuid.UUID
	if raw := strings.TrimSpace(r.FormValue("series_id")); raw != "" {
		sid, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid series_id")
			return
		}
		if _, err := a.Store.GetSeries(r.Context(), sid); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "series not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to validate series")
			return
		}
		seriesID = &sid
	}

	season, err := parseOptionalInt(r.FormValue("season"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid season")
		return
	}
	episode, err := parseOptionalInt(r.FormValue("episode"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode")
		return
	}
	sortOrder := existing.SortOrder
	if raw := strings.TrimSpace(r.FormValue("sort_order")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sort_order")
			return
		}
		sortOrder = n
	} else if _, ok := r.Form["sort_order"]; ok {
		sortOrder = 0
	}

	posterRel := existing.PosterPath
	var newPosterAbs string
	posterFile, _, err := r.FormFile("poster")
	if err == nil {
		defer posterFile.Close()
		rel := filepath.Join("posters", id.String()+".webp")
		abs := filepath.Join(a.MediaRoot, rel)
		if err := saveImageAsWebP(posterFile, abs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save poster")
			return
		}
		posterRel = rel
		newPosterAbs = abs
	}

	existing.Name = name
	existing.Description = description
	existing.PosterPath = posterRel
	existing.SeriesID = seriesID
	existing.Season = season
	existing.Episode = episode
	existing.SortOrder = sortOrder
	if err := a.Store.Update(r.Context(), existing); err != nil {
		if newPosterAbs != "" {
			_ = os.Remove(newPosterAbs)
		}
		writeError(w, http.StatusInternalServerError, "failed to update animation")
		return
	}

	updated, err := a.Store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload animation")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) TranscodeAnimation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}
	if item.PlaybackStatus == "processing" {
		writeJSON(w, http.StatusAccepted, item)
		return
	}
	if err := a.Store.SetPlaybackStatus(r.Context(), id, "processing"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue transcode")
		return
	}
	a.StartTranscode(id)
	item.PlaybackStatus = "processing"
	writeJSON(w, http.StatusAccepted, item)
}

// StartTranscode converts a stored file to browser-friendly H.264/AAC MP4 in the background.
func (a *API) StartTranscode(id uuid.UUID) {
	key := id.String()
	if _, loaded := a.transcoding.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer a.transcoding.Delete(key)
		ctx := context.Background()
		item, err := a.Store.Get(ctx, id)
		if err != nil {
			log.Printf("transcode %s: load: %v", id, err)
			return
		}
		src := filepath.Join(a.MediaRoot, item.VideoPath)
		_ = a.Store.SetPlaybackStatus(ctx, id, "processing")
		dest, err := transcode.ReplaceWithMP4(src)
		if err != nil {
			log.Printf("transcode %s: %v", id, err)
			_ = a.Store.SetPlaybackStatus(ctx, id, "failed")
			return
		}
		rel, err := filepath.Rel(a.MediaRoot, dest)
		if err != nil {
			rel = filepath.Join("videos", filepath.Base(dest))
		}
		if err := a.Store.UpdatePlayback(ctx, id, rel, "video/mp4", "ready"); err != nil {
			log.Printf("transcode %s: update db: %v", id, err)
			_ = a.Store.SetPlaybackStatus(ctx, id, "failed")
			return
		}
		log.Printf("transcode %s: ready (%s)", id, rel)
	}()
}

// QueuePendingTranscodes resumes conversions after restart.
func (a *API) QueuePendingTranscodes() {
	items, err := a.Store.ListNeedingTranscode(context.Background())
	if err != nil {
		log.Printf("queue transcodes: %v", err)
		return
	}
	for _, item := range items {
		if item.PlaybackStatus == "ready" && !transcode.NeedsTranscode(filepath.Ext(item.VideoPath), item.ContentType) {
			continue
		}
		log.Printf("queue transcode for %s (%s)", item.ID, item.Name)
		_ = a.Store.SetPlaybackStatus(context.Background(), item.ID, "processing")
		a.StartTranscode(item.ID)
	}
}

func (a *API) DeleteAnimation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	_ = os.Remove(filepath.Join(a.MediaRoot, item.VideoPath))
	_ = os.Remove(filepath.Join(a.MediaRoot, item.PosterPath))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ServePoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}
	path := filepath.Join(a.MediaRoot, item.PosterPath)
	if strings.HasSuffix(strings.ToLower(path), ".webp") {
		w.Header().Set("Content-Type", "image/webp")
	}
	http.ServeFile(w, r, path)
}

func (a *API) StreamVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}
	if item.PlaybackStatus == "processing" {
		writeError(w, http.StatusConflict, "video is still being converted for browser playback")
		return
	}
	if item.PlaybackStatus == "failed" {
		writeError(w, http.StatusUnsupportedMediaType, "conversion failed — re-upload as H.264 MP4 or retry transcode")
		return
	}

	path := filepath.Join(a.MediaRoot, item.VideoPath)
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "video file missing")
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read video")
		return
	}

	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, item.Name+filepath.Ext(item.VideoPath), stat.ModTime(), f)
}

func parseOptionalInt(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func saveUpload(src io.Reader, dest string) error {
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
}

func saveImageAsWebP(src io.Reader, destWebP string) error {
	if err := os.MkdirAll(filepath.Dir(destWebP), 0o755); err != nil {
		return err
	}
	rawTmp := destWebP + ".src.tmp"
	if err := saveUpload(src, rawTmp); err != nil {
		return err
	}
	defer os.Remove(rawTmp)
	return imageconv.SaveUploadAsWebP(rawTmp, destWebP)
}

func extOr(filename, fallback string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return fallback
	}
	return ext
}

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	default:
		return "video/mp4"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
