package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/spread/thousand_sunny/internal/models"
	"github.com/spread/thousand_sunny/internal/store"
)

const maxUpload = 4 << 30 // 4 GiB

type API struct {
	Store     *store.Store
	MediaRoot string
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/animations", a.ListAnimations)
	r.Post("/animations", a.CreateAnimation)
	r.Get("/animations/{id}", a.GetAnimation)
	r.Delete("/animations/{id}", a.DeleteAnimation)
	r.Get("/animations/{id}/poster", a.ServePoster)
	r.Get("/animations/{id}/stream", a.StreamVideo)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return r
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

	videoFile, videoHeader, err := r.FormFile("video")
	if err != nil {
		writeError(w, http.StatusBadRequest, "video file is required")
		return
	}
	defer videoFile.Close()

	posterFile, posterHeader, err := r.FormFile("poster")
	if err != nil {
		writeError(w, http.StatusBadRequest, "poster file is required")
		return
	}
	defer posterFile.Close()

	id := uuid.New()
	videoExt := extOr(videoHeader.Filename, ".mp4")
	posterExt := extOr(posterHeader.Filename, ".jpg")
	videoRel := filepath.Join("videos", id.String()+videoExt)
	posterRel := filepath.Join("posters", id.String()+posterExt)
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
	if err := saveUpload(posterFile, posterAbs); err != nil {
		_ = os.Remove(videoAbs)
		writeError(w, http.StatusInternalServerError, "failed to save poster")
		return
	}

	contentType := videoHeader.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeFromExt(videoExt)
	}

	anim := &models.Animation{
		ID:          id,
		Name:        name,
		Description: description,
		VideoPath:   videoRel,
		PosterPath:  posterRel,
		ContentType: contentType,
	}
	if err := a.Store.Create(r.Context(), anim); err != nil {
		_ = os.Remove(videoAbs)
		_ = os.Remove(posterAbs)
		writeError(w, http.StatusInternalServerError, "failed to save animation")
		return
	}
	anim.WithURLs()
	writeJSON(w, http.StatusCreated, anim)
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
