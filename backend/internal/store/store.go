package store

import (
	"context"
	"fmt"
	"strings"

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
