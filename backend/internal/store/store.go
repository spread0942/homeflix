package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/spread/thousand_sunny/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) List(ctx context.Context, q string) ([]models.Animation, error) {
	q = strings.TrimSpace(q)
	var (
		rows pgx.Rows
		err  error
	)

	if q == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, name, description, video_path, poster_path, content_type, created_at
			FROM animations
			ORDER BY created_at DESC`)
	} else {
		pattern := "%" + q + "%"
		rows, err = s.pool.Query(ctx, `
			SELECT id, name, description, video_path, poster_path, content_type, created_at
			FROM animations
			WHERE name ILIKE $1 OR description ILIKE $1
			ORDER BY created_at DESC`, pattern)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Animation
	for rows.Next() {
		var a models.Animation
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.VideoPath, &a.PosterPath, &a.ContentType, &a.CreatedAt); err != nil {
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

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*models.Animation, error) {
	var a models.Animation
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, description, video_path, poster_path, content_type, created_at
		FROM animations WHERE id = $1`, id).
		Scan(&a.ID, &a.Name, &a.Description, &a.VideoPath, &a.PosterPath, &a.ContentType, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.WithURLs()
	return &a, nil
}

func (s *Store) Create(ctx context.Context, a *models.Animation) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO animations (id, name, description, video_path, poster_path, content_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`,
		a.ID, a.Name, a.Description, a.VideoPath, a.PosterPath, a.ContentType,
	).Scan(&a.CreatedAt)
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
