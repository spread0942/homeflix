package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/spread/homeflix/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const animationSelect = `
	a.id, a.name, a.description, a.video_path, a.poster_path, a.content_type, a.playback_status,
	a.series_id, COALESCE(s.name, ''), a.season, a.episode, a.sort_order, a.created_at`

func scanAnimation(row pgx.Row) (*models.Animation, error) {
	var a models.Animation
	err := row.Scan(
		&a.ID, &a.Name, &a.Description, &a.VideoPath, &a.PosterPath, &a.ContentType, &a.PlaybackStatus,
		&a.SeriesID, &a.SeriesName, &a.Season, &a.Episode, &a.SortOrder, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.WithURLs()
	return &a, nil
}

func scanAnimationRows(rows pgx.Rows) ([]models.Animation, error) {
	var out []models.Animation
	for rows.Next() {
		var a models.Animation
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.VideoPath, &a.PosterPath, &a.ContentType, &a.PlaybackStatus,
			&a.SeriesID, &a.SeriesName, &a.Season, &a.Episode, &a.SortOrder, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.WithURLs()
		out = append(out, a)
	}
	if out == nil {
		out = []models.Animation{}
	}
	return out, rows.Err()
}

func (s *Store) List(ctx context.Context, q string) ([]models.Animation, error) {
	q = strings.TrimSpace(q)
	var (
		rows pgx.Rows
		err  error
	)

	base := `
		SELECT ` + animationSelect + `
		FROM animations a
		LEFT JOIN series s ON s.id = a.series_id`

	if q == "" {
		rows, err = s.pool.Query(ctx, base+` ORDER BY a.created_at DESC`)
	} else {
		pattern := "%" + q + "%"
		rows, err = s.pool.Query(ctx, base+`
			WHERE a.name ILIKE $1 OR a.description ILIKE $1 OR s.name ILIKE $1
			ORDER BY a.created_at DESC`, pattern)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimationRows(rows)
}

func (s *Store) ListStandalone(ctx context.Context, q string) ([]models.Animation, error) {
	q = strings.TrimSpace(q)
	var (
		rows pgx.Rows
		err  error
	)
	base := `
		SELECT ` + animationSelect + `
		FROM animations a
		LEFT JOIN series s ON s.id = a.series_id
		WHERE a.series_id IS NULL`
	if q == "" {
		rows, err = s.pool.Query(ctx, base+` ORDER BY a.created_at DESC`)
	} else {
		pattern := "%" + q + "%"
		rows, err = s.pool.Query(ctx, base+`
			AND (a.name ILIKE $1 OR a.description ILIKE $1)
			ORDER BY a.created_at DESC`, pattern)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimationRows(rows)
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*models.Animation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+animationSelect+`
		FROM animations a
		LEFT JOIN series s ON s.id = a.series_id
		WHERE a.id = $1`, id)
	return scanAnimation(row)
}

func (s *Store) Create(ctx context.Context, a *models.Animation) error {
	if a.PlaybackStatus == "" {
		a.PlaybackStatus = "ready"
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO animations (
			id, name, description, video_path, poster_path, content_type, playback_status,
			series_id, season, episode, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at`,
		a.ID, a.Name, a.Description, a.VideoPath, a.PosterPath, a.ContentType, a.PlaybackStatus,
		a.SeriesID, a.Season, a.Episode, a.SortOrder,
	).Scan(&a.CreatedAt)
}

func (s *Store) Update(ctx context.Context, a *models.Animation) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE animations
		SET name = $2, description = $3, poster_path = $4,
			series_id = $5, season = $6, episode = $7, sort_order = $8
		WHERE id = $1`,
		a.ID, a.Name, a.Description, a.PosterPath,
		a.SeriesID, a.Season, a.Episode, a.SortOrder,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) UpdatePlayback(ctx context.Context, id uuid.UUID, videoPath, contentType, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE animations
		SET video_path = $2, content_type = $3, playback_status = $4
		WHERE id = $1`, id, videoPath, contentType, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetPlaybackStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE animations SET playback_status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ListNeedingTranscode(ctx context.Context) ([]models.Animation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+animationSelect+`
		FROM animations a
		LEFT JOIN series s ON s.id = a.series_id
		WHERE a.playback_status IN ('processing', 'failed')
			OR a.video_path ILIKE '%.mkv'
			OR a.video_path ILIKE '%.avi'
			OR a.video_path ILIKE '%.mov'
			OR a.video_path ILIKE '%.wmv'
			OR a.video_path ILIKE '%.ts'
			OR a.video_path ILIKE '%.m2ts'
			OR a.content_type ILIKE '%matroska%'
		ORDER BY a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimationRows(rows)
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) (*models.Animation, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM animations WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return a, nil
}

func (s *Store) ListBySeries(ctx context.Context, seriesID uuid.UUID) ([]models.Animation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+animationSelect+`
		FROM animations a
		LEFT JOIN series s ON s.id = a.series_id
		WHERE a.series_id = $1
		ORDER BY a.season NULLS FIRST, a.episode NULLS FIRST, a.sort_order, a.created_at`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimationRows(rows)
}

func (s *Store) ListSeries(ctx context.Context, q string) ([]models.Series, error) {
	q = strings.TrimSpace(q)
	var (
		rows pgx.Rows
		err  error
	)
	if q == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT s.id, s.name, s.description, s.poster_path, s.kind, s.created_at,
				COUNT(a.id)::int AS entry_count,
				(
					SELECT a2.poster_path FROM animations a2
					WHERE a2.series_id = s.id
					ORDER BY a2.season NULLS FIRST, a2.episode NULLS FIRST, a2.sort_order, a2.created_at
					LIMIT 1
				) AS fallback_poster
			FROM series s
			LEFT JOIN animations a ON a.series_id = s.id
			GROUP BY s.id
			ORDER BY s.created_at DESC`)
	} else {
		pattern := "%" + q + "%"
		rows, err = s.pool.Query(ctx, `
			SELECT s.id, s.name, s.description, s.poster_path, s.kind, s.created_at,
				COUNT(a.id)::int AS entry_count,
				(
					SELECT a2.poster_path FROM animations a2
					WHERE a2.series_id = s.id
					ORDER BY a2.season NULLS FIRST, a2.episode NULLS FIRST, a2.sort_order, a2.created_at
					LIMIT 1
				) AS fallback_poster
			FROM series s
			LEFT JOIN animations a ON a.series_id = s.id
			WHERE s.name ILIKE $1 OR s.description ILIKE $1
				OR EXISTS (
					SELECT 1 FROM animations ax
					WHERE ax.series_id = s.id AND (ax.name ILIKE $1 OR ax.description ILIKE $1)
				)
			GROUP BY s.id
			ORDER BY s.created_at DESC`, pattern)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Series
	for rows.Next() {
		var ser models.Series
		var fallback *string
		if err := rows.Scan(
			&ser.ID, &ser.Name, &ser.Description, &ser.PosterPath, &ser.Kind, &ser.CreatedAt,
			&ser.EntryCount, &fallback,
		); err != nil {
			return nil, err
		}
		applySeriesPoster(&ser, fallback)
		out = append(out, ser)
	}
	if out == nil {
		out = []models.Series{}
	}
	return out, rows.Err()
}

func (s *Store) GetSeries(ctx context.Context, id uuid.UUID) (*models.Series, error) {
	var ser models.Series
	var fallback *string
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.name, s.description, s.poster_path, s.kind, s.created_at,
			(SELECT COUNT(*)::int FROM animations a WHERE a.series_id = s.id),
			(
				SELECT a2.poster_path FROM animations a2
				WHERE a2.series_id = s.id
				ORDER BY a2.season NULLS FIRST, a2.episode NULLS FIRST, a2.sort_order, a2.created_at
				LIMIT 1
			)
		FROM series s WHERE s.id = $1`, id).
		Scan(&ser.ID, &ser.Name, &ser.Description, &ser.PosterPath, &ser.Kind, &ser.CreatedAt, &ser.EntryCount, &fallback)
	if err != nil {
		return nil, err
	}
	applySeriesPoster(&ser, fallback)
	entries, err := s.ListBySeries(ctx, id)
	if err != nil {
		return nil, err
	}
	ser.Entries = entries
	return &ser, nil
}

func (s *Store) CreateSeries(ctx context.Context, ser *models.Series) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO series (id, name, description, poster_path, kind)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`,
		ser.ID, ser.Name, ser.Description, ser.PosterPath, ser.Kind,
	).Scan(&ser.CreatedAt)
}

func (s *Store) UpdateSeries(ctx context.Context, ser *models.Series) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE series
		SET name = $2, description = $3, poster_path = $4, kind = $5
		WHERE id = $1`,
		ser.ID, ser.Name, ser.Description, ser.PosterPath, ser.Kind,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteSeries(ctx context.Context, id uuid.UUID) (*models.Series, []models.Animation, error) {
	ser, err := s.GetSeries(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	entries := ser.Entries
	tag, err := s.pool.Exec(ctx, `DELETE FROM series WHERE id = $1`, id)
	if err != nil {
		return nil, nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil, pgx.ErrNoRows
	}
	return ser, entries, nil
}

func (s *Store) Library(ctx context.Context, q string) ([]models.LibraryItem, error) {
	seriesList, err := s.ListSeries(ctx, q)
	if err != nil {
		return nil, err
	}
	films, err := s.ListStandalone(ctx, q)
	if err != nil {
		return nil, err
	}

	items := make([]models.LibraryItem, 0, len(seriesList)+len(films))
	for _, ser := range seriesList {
		items = append(items, models.LibraryItem{
			Type:        "series",
			ID:          ser.ID,
			Name:        ser.Name,
			Description: ser.Description,
			PosterURL:   ser.PosterURL,
			EntryCount:  ser.EntryCount,
			Kind:        ser.Kind,
			CreatedAt:   ser.CreatedAt,
		})
	}
	for _, f := range films {
		id := f.ID
		items = append(items, models.LibraryItem{
			Type:        "film",
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			PosterURL:   f.PosterURL,
			CreatedAt:   f.CreatedAt,
			FilmID:      &id,
		})
	}

	// newest first across both types
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt.After(items[i].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items, nil
}

func applySeriesPoster(ser *models.Series, fallback *string) {
	if ser.PosterPath != nil && *ser.PosterPath != "" {
		ser.WithPosterURL()
		return
	}
	if fallback != nil && *fallback != "" {
		ser.PosterURL = "/api/series/" + ser.ID.String() + "/poster"
	}
}

func (s *Store) GetProgress(ctx context.Context, animationID uuid.UUID) (*models.WatchProgress, error) {
	var p models.WatchProgress
	err := s.pool.QueryRow(ctx, `
		SELECT animation_id, position_seconds, duration_seconds, completed, updated_at
		FROM watch_progress WHERE animation_id = $1`, animationID).
		Scan(&p.AnimationID, &p.PositionSeconds, &p.DurationSeconds, &p.Completed, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ErrSkipProgress means the client sent a negligible position and there is nothing to store.
var ErrSkipProgress = errors.New("skip progress")

func (s *Store) UpsertProgress(ctx context.Context, p *models.WatchProgress) error {
	const minKeepSeconds = 5.0

	existing, err := s.GetProgress(ctx, p.AnimationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Ignore near-zero "saves" that would wipe a real resume point (common on
	// unmount/pagehide when the video element has already reset).
	if existing != nil && !p.Completed && p.PositionSeconds < minKeepSeconds &&
		existing.PositionSeconds >= minKeepSeconds && !existing.Completed {
		*p = *existing
		return nil
	}
	if !p.Completed && p.PositionSeconds < minKeepSeconds {
		if existing == nil {
			return ErrSkipProgress
		}
		*p = *existing
		return nil
	}

	if existing != nil && p.DurationSeconds <= 0 && existing.DurationSeconds > 0 {
		p.DurationSeconds = existing.DurationSeconds
	}

	return s.pool.QueryRow(ctx, `
		INSERT INTO watch_progress (animation_id, position_seconds, duration_seconds, completed, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (animation_id) DO UPDATE SET
			position_seconds = EXCLUDED.position_seconds,
			duration_seconds = EXCLUDED.duration_seconds,
			completed = EXCLUDED.completed,
			updated_at = now()
		RETURNING updated_at`,
		p.AnimationID, p.PositionSeconds, p.DurationSeconds, p.Completed,
	).Scan(&p.UpdatedAt)
}

type progressRow struct {
	progress     models.WatchProgress
	name         string
	posterPath   string
	status       string
	seriesID     *uuid.UUID
	seriesName   string
	seriesPoster *string
	season       *int
	episode      *int
	sortOrder    int
}

func (s *Store) ListContinueWatching(ctx context.Context, limit int) ([]models.ContinueWatchingItem, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			wp.animation_id, wp.position_seconds, wp.duration_seconds, wp.completed, wp.updated_at,
			a.name, a.poster_path, a.playback_status, a.series_id, a.season, a.episode, a.sort_order,
			COALESCE(s.name, ''), s.poster_path
		FROM watch_progress wp
		JOIN animations a ON a.id = wp.animation_id
		LEFT JOIN series s ON s.id = a.series_id
		ORDER BY wp.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rowsData []progressRow
	for rows.Next() {
		var r progressRow
		if err := rows.Scan(
			&r.progress.AnimationID, &r.progress.PositionSeconds, &r.progress.DurationSeconds,
			&r.progress.Completed, &r.progress.UpdatedAt,
			&r.name, &r.posterPath, &r.status, &r.seriesID, &r.season, &r.episode, &r.sortOrder,
			&r.seriesName, &r.seriesPoster,
		); err != nil {
			return nil, err
		}
		rowsData = append(rowsData, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	out := make([]models.ContinueWatchingItem, 0, limit)
	seriesCache := make(map[uuid.UUID][]models.Animation)

	for _, r := range rowsData {
		key := "film:" + r.progress.AnimationID.String()
		if r.seriesID != nil {
			key = "series:" + r.seriesID.String()
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		if r.seriesID == nil {
			if r.progress.Completed {
				continue
			}
			out = append(out, continueItemFromRow(r, false))
		} else if r.progress.Completed {
			entries, ok := seriesCache[*r.seriesID]
			if !ok {
				entries, err = s.ListBySeries(ctx, *r.seriesID)
				if err != nil {
					return nil, err
				}
				seriesCache[*r.seriesID] = entries
			}
			next := nextPlayableAfter(entries, r.progress.AnimationID)
			if next != nil {
				item := continueItemFromAnimation(next, r.seriesName, r.seriesPoster, r.progress.UpdatedAt)
				out = append(out, item)
			} else {
				item := continueItemFromRow(r, true)
				out = append(out, item)
			}
		} else {
			out = append(out, continueItemFromRow(r, false))
		}

		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func nextPlayableAfter(entries []models.Animation, currentID uuid.UUID) *models.Animation {
	idx := -1
	for i := range entries {
		if entries[i].ID == currentID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	for j := idx + 1; j < len(entries); j++ {
		if entries[j].PlaybackStatus == "ready" {
			return &entries[j]
		}
	}
	return nil
}

func continueItemFromRow(r progressRow, seriesComplete bool) models.ContinueWatchingItem {
	item := models.ContinueWatchingItem{
		AnimationID:     r.progress.AnimationID,
		Name:            r.name,
		PositionSeconds: r.progress.PositionSeconds,
		DurationSeconds: r.progress.DurationSeconds,
		Completed:       r.progress.Completed,
		UpdatedAt:       r.progress.UpdatedAt,
		SeriesID:        r.seriesID,
		SeriesName:      r.seriesName,
		Season:          r.season,
		Episode:         r.episode,
		EntryLabel:      models.EntryLabel(r.season, r.episode, r.sortOrder),
		SeriesComplete:  seriesComplete,
	}
	item.PosterURL = continuePosterURL(r.seriesID, r.seriesPoster, r.progress.AnimationID, r.posterPath)
	return item
}

func continueItemFromAnimation(a *models.Animation, seriesName string, seriesPoster *string, updatedAt time.Time) models.ContinueWatchingItem {
	item := models.ContinueWatchingItem{
		AnimationID:     a.ID,
		Name:            a.Name,
		PositionSeconds: 0,
		DurationSeconds: 0,
		Completed:       false,
		UpdatedAt:       updatedAt,
		SeriesID:        a.SeriesID,
		SeriesName:      seriesName,
		Season:          a.Season,
		Episode:         a.Episode,
		EntryLabel:      models.EntryLabel(a.Season, a.Episode, a.SortOrder),
		SeriesComplete:  false,
	}
	item.PosterURL = continuePosterURL(a.SeriesID, seriesPoster, a.ID, a.PosterPath)
	return item
}

func continuePosterURL(seriesID *uuid.UUID, seriesPoster *string, animationID uuid.UUID, animPoster string) string {
	if seriesID != nil && seriesPoster != nil && *seriesPoster != "" {
		return "/api/series/" + seriesID.String() + "/poster"
	}
	if seriesID != nil && animPoster != "" {
		// series may use first-entry fallback via series poster endpoint
		return "/api/series/" + seriesID.String() + "/poster"
	}
	if animPoster != "" {
		return "/api/animations/" + animationID.String() + "/poster"
	}
	return ""
}

// SeriesPosterFile returns the absolute-relative poster path for a series (own or first entry).
func (s *Store) SeriesPosterFile(ctx context.Context, id uuid.UUID) (string, error) {
	var own *string
	err := s.pool.QueryRow(ctx, `SELECT poster_path FROM series WHERE id = $1`, id).Scan(&own)
	if err != nil {
		return "", err
	}
	if own != nil && *own != "" {
		return *own, nil
	}
	var fallback *string
	err = s.pool.QueryRow(ctx, `
		SELECT poster_path FROM animations
		WHERE series_id = $1
		ORDER BY season NULLS FIRST, episode NULLS FIRST, sort_order, created_at
		LIMIT 1`, id).Scan(&fallback)
	if err != nil {
		return "", err
	}
	if fallback == nil || *fallback == "" {
		return "", pgx.ErrNoRows
	}
	return *fallback, nil
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}
