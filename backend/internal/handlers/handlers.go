package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/spread/homeflix/internal/subtitles"
	"github.com/spread/homeflix/internal/transcode"
)

const (
	maxFormFieldBytes = 1 << 20
	maxPosterBytes    = 32 << 20
	maxSubtitleBytes  = 16 << 20
)

var errPartTooLarge = errors.New("form part is too large")

type API struct {
	Store          *store.Store
	MediaRoot      string
	MaxUploadBytes int64

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
	r.Get("/animations/{id}/subtitles/{subtitleId}", a.ServeSubtitle)
	r.Post("/animations/{id}/subtitles", a.AddSubtitle)
	r.Delete("/animations/{id}/subtitles/{subtitleId}", a.DeleteSubtitle)
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
		if errors.Is(err, store.ErrSkipProgress) {
			writeJSON(w, http.StatusOK, p)
			return
		}
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
		for _, t := range e.Subtitles {
			if t.FilePath != "" {
				_ = os.Remove(filepath.Join(a.MediaRoot, t.FilePath))
			}
		}
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

type animationMeta struct {
	name        string
	description string
	seriesID    *uuid.UUID
	season      *int
	episode     *int
	sortOrder   int
}

func (a *API) parseAnimationMeta(ctx context.Context, fields map[string]string) (*animationMeta, int, string) {
	m := &animationMeta{
		name:        strings.TrimSpace(fields["name"]),
		description: strings.TrimSpace(fields["description"]),
	}
	if m.name == "" {
		return nil, http.StatusBadRequest, "name is required"
	}
	if raw := strings.TrimSpace(fields["series_id"]); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, http.StatusBadRequest, "invalid series_id"
		}
		if _, err := a.Store.GetSeries(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, http.StatusBadRequest, "series not found"
			}
			return nil, http.StatusInternalServerError, "failed to validate series"
		}
		m.seriesID = &id
	}
	var err error
	if m.season, err = parseOptionalInt(fields["season"]); err != nil {
		return nil, http.StatusBadRequest, "invalid season"
	}
	if m.episode, err = parseOptionalInt(fields["episode"]); err != nil {
		return nil, http.StatusBadRequest, "invalid episode"
	}
	if raw := strings.TrimSpace(fields["sort_order"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, http.StatusBadRequest, "invalid sort_order"
		}
		m.sortOrder = n
	}
	return m, 0, ""
}

type uploadedSubtitle struct {
	filename string
	data     []byte
}

// CreateAnimation streams the multipart body so multi-GB videos are written
// once, directly under MediaRoot. Text fields must be sent before the video part.
func (a *API) CreateAnimation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	id := uuid.New()
	fields := map[string]string{}
	var (
		meta     *animationMeta
		videoExt string
		videoCT  string
		videoRel string
		videoAbs string
		poster   []byte
		subFiles []uploadedSubtitle
		subLangs []string
	)
	fail := func(status int, msg string) {
		if videoAbs != "" {
			_ = os.Remove(videoAbs)
		}
		writeError(w, status, msg)
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(a.uploadReadError(err, http.StatusBadRequest, "invalid multipart form"))
			return
		}

		switch name := part.FormName(); {
		case name == "video":
			if meta != nil {
				fail(http.StatusBadRequest, "only one video file is allowed")
				return
			}
			m, status, msg := a.parseAnimationMeta(r.Context(), fields)
			if m == nil {
				fail(status, msg)
				return
			}
			meta = m
			videoExt = extOr(part.FileName(), ".mp4")
			videoCT = part.Header.Get("Content-Type")
			videoRel = filepath.Join("videos", id.String()+videoExt)
			dest := filepath.Join(a.MediaRoot, videoRel)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				fail(http.StatusInternalServerError, "failed to prepare storage")
				return
			}
			if err := saveUpload(part, dest); err != nil {
				fail(a.uploadReadError(err, http.StatusInternalServerError, "failed to save video"))
				return
			}
			videoAbs = dest
		case name == "poster":
			data, err := readPart(part, maxPosterBytes)
			if err != nil {
				fail(a.uploadReadError(err, http.StatusBadRequest, "failed to read poster"))
				return
			}
			if len(data) > 0 {
				poster = data
			}
		case name == "subtitle":
			data, err := readPart(part, maxSubtitleBytes)
			if err != nil {
				fail(a.uploadReadError(err, http.StatusBadRequest, "failed to read subtitle"))
				return
			}
			if part.FileName() != "" {
				subFiles = append(subFiles, uploadedSubtitle{filename: part.FileName(), data: data})
			}
		case part.FileName() != "":
		default:
			data, err := readPart(part, maxFormFieldBytes)
			if err != nil {
				fail(a.uploadReadError(err, http.StatusBadRequest, "invalid multipart form"))
				return
			}
			if name == "subtitle_lang" {
				subLangs = append(subLangs, string(data))
			} else if _, ok := fields[name]; !ok {
				fields[name] = string(data)
			}
		}
		part.Close()
	}

	if meta == nil {
		writeError(w, http.StatusBadRequest, "video file is required")
		return
	}

	pendingSubs := make([]pendingSubtitle, 0, len(subFiles))
	for i, s := range subFiles {
		lang := ""
		if i < len(subLangs) {
			lang = subLangs[i]
		}
		p, err := newPendingSubtitle(s.filename, lang, io.NopCloser(bytes.NewReader(s.data)))
		if err != nil {
			fail(http.StatusBadRequest, err.Error())
			return
		}
		pendingSubs = append(pendingSubs, p)
	}

	posterRel := filepath.Join("posters", id.String()+".webp")
	posterAbs := filepath.Join(a.MediaRoot, posterRel)
	if err := os.MkdirAll(filepath.Dir(posterAbs), 0o755); err != nil {
		fail(http.StatusInternalServerError, "failed to prepare storage")
		return
	}
	if poster != nil {
		if err := saveImageAsWebP(bytes.NewReader(poster), posterAbs); err != nil {
			fail(http.StatusInternalServerError, "failed to save poster")
			return
		}
	} else if err := imageconv.ExtractFrameAsWebP(videoAbs, posterAbs); err != nil {
		log.Printf("auto poster for %s: %v", id, err)
		fail(http.StatusInternalServerError, "failed to generate poster from video")
		return
	}

	savedSubPaths := make([]string, 0, len(pendingSubs))
	cleanupCreate := func() {
		_ = os.Remove(videoAbs)
		_ = os.Remove(posterAbs)
		for _, p := range savedSubPaths {
			_ = os.Remove(p)
		}
	}

	for i := range pendingSubs {
		trackID := uuid.New()
		ext := pendingSubs[i].ext
		rel := filepath.Join("subtitles", id.String()+"_"+pendingSubs[i].lang+"_"+trackID.String()[:8]+ext)
		abs := filepath.Join(a.MediaRoot, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			cleanupCreate()
			writeError(w, http.StatusInternalServerError, "failed to prepare storage")
			return
		}
		if err := saveUpload(pendingSubs[i].file, abs); err != nil {
			cleanupCreate()
			writeError(w, http.StatusInternalServerError, "failed to save subtitle")
			return
		}
		pendingSubs[i].file.Close()
		pendingSubs[i].rel = rel
		pendingSubs[i].abs = abs
		pendingSubs[i].id = trackID
		savedSubPaths = append(savedSubPaths, abs)
	}

	contentType := videoCT
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeFromExt(videoExt)
	}

	status := "ready"
	if transcode.NeedsTranscode(videoExt, contentType) {
		status = "processing"
	}

	anim := &models.Animation{
		ID:             id,
		Name:           meta.name,
		Description:    meta.description,
		VideoPath:      videoRel,
		PosterPath:     posterRel,
		ContentType:    contentType,
		PlaybackStatus: status,
		SeriesID:       meta.seriesID,
		Season:         meta.season,
		Episode:        meta.episode,
		SortOrder:      meta.sortOrder,
	}
	if err := a.Store.Create(r.Context(), anim); err != nil {
		cleanupCreate()
		writeError(w, http.StatusInternalServerError, "failed to save animation")
		return
	}
	for _, p := range pendingSubs {
		track := &models.SubtitleTrack{
			ID:          p.id,
			AnimationID: id,
			Language:    p.lang,
			Label:       p.label,
			FilePath:    p.rel,
		}
		if _, err := a.Store.UpsertSubtitle(r.Context(), track); err != nil {
			cleanupCreate()
			if deleted, delErr := a.Store.Delete(r.Context(), id); delErr == nil && deleted != nil {
				_ = os.Remove(filepath.Join(a.MediaRoot, deleted.VideoPath))
				_ = os.Remove(filepath.Join(a.MediaRoot, deleted.PosterPath))
			}
			writeError(w, http.StatusInternalServerError, "failed to save subtitle")
			return
		}
	}
	if status == "processing" {
		a.StartTranscode(id)
	}
	// reload for series_name + subtitles
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
	for _, t := range item.Subtitles {
		if t.FilePath != "" {
			_ = os.Remove(filepath.Join(a.MediaRoot, t.FilePath))
		}
	}
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

func (a *API) ServeSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	subID, err := uuid.Parse(chi.URLParam(r, "subtitleId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid subtitle id")
		return
	}
	track, err := a.Store.GetSubtitle(r.Context(), id, subID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get subtitle")
		return
	}
	path := filepath.Join(a.MediaRoot, track.FilePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "subtitle file missing")
		return
	}
	vtt := subtitles.ToVTT(raw)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(vtt)
}

func (a *API) AddSubtitle(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusInternalServerError, "failed to get animation")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	pending, err := a.readSubtitleUploads(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(pending) == 0 {
		writeError(w, http.StatusBadRequest, "subtitle file is required")
		return
	}

	var created []models.SubtitleTrack
	for i := range pending {
		p := &pending[i]
		trackID := uuid.New()
		rel := filepath.Join("subtitles", id.String()+"_"+p.lang+"_"+trackID.String()[:8]+p.ext)
		abs := filepath.Join(a.MediaRoot, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			p.file.Close()
			writeError(w, http.StatusInternalServerError, "failed to prepare storage")
			return
		}
		if err := saveUpload(p.file, abs); err != nil {
			p.file.Close()
			writeError(w, http.StatusInternalServerError, "failed to save subtitle")
			return
		}
		p.file.Close()

		track := &models.SubtitleTrack{
			ID:          trackID,
			AnimationID: id,
			Language:    p.lang,
			Label:       p.label,
			FilePath:    rel,
		}
		oldPath, err := a.Store.UpsertSubtitle(r.Context(), track)
		if err != nil {
			_ = os.Remove(abs)
			writeError(w, http.StatusInternalServerError, "failed to save subtitle")
			return
		}
		if oldPath != "" && oldPath != rel {
			_ = os.Remove(filepath.Join(a.MediaRoot, oldPath))
		}
		created = append(created, *track)
	}

	if len(created) == 1 {
		writeJSON(w, http.StatusCreated, created[0])
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) DeleteSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	subID, err := uuid.Parse(chi.URLParam(r, "subtitleId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid subtitle id")
		return
	}
	track, err := a.Store.DeleteSubtitle(r.Context(), id, subID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete subtitle")
		return
	}
	if track.FilePath != "" {
		_ = os.Remove(filepath.Join(a.MediaRoot, track.FilePath))
	}
	w.WriteHeader(http.StatusNoContent)
}

type pendingSubtitle struct {
	file  multipartFile
	ext   string
	lang  string
	label string
	id    uuid.UUID
	rel   string
	abs   string
}

type multipartFile interface {
	io.Reader
	io.Closer
}

func (a *API) readSubtitleUploads(r *http.Request) ([]pendingSubtitle, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File["subtitle"]
	if len(headers) == 0 {
		return nil, nil
	}
	langs := r.MultipartForm.Value["subtitle_lang"]
	out := make([]pendingSubtitle, 0, len(headers))
	for i, h := range headers {
		f, err := h.Open()
		if err != nil {
			return nil, errors.New("failed to read subtitle")
		}
		lang := ""
		if i < len(langs) {
			lang = langs[i]
		}
		p, err := newPendingSubtitle(h.Filename, lang, f)
		if err != nil {
			f.Close()
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func newPendingSubtitle(filename, langRaw string, file multipartFile) (pendingSubtitle, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".srt" && ext != ".vtt" {
		return pendingSubtitle{}, errors.New("subtitle must be .srt or .vtt")
	}
	p := pendingSubtitle{file: file, ext: ext}
	if strings.TrimSpace(langRaw) != "" {
		p.lang = subtitles.NormalizeLang(langRaw)
		p.label = subtitles.LabelFor(p.lang)
	} else {
		_, p.lang, p.label = subtitles.ParseFilename(filename)
	}
	return p, nil
}

func readPart(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errPartTooLarge
	}
	return data, nil
}

func (a *API) uploadReadError(err error, status int, msg string) (int, string) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return http.StatusRequestEntityTooLarge, fmt.Sprintf("upload exceeds the %d GiB limit (MAX_UPLOAD_GB)", a.MaxUploadBytes>>30)
	}
	if errors.Is(err, errPartTooLarge) {
		return http.StatusBadRequest, err.Error()
	}
	return status, msg
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
