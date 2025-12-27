package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SQLRepository struct {
	pool *pgxpool.Pool
}

func newSQLRepository(pool *pgxpool.Pool) *SQLRepository {
	return &SQLRepository{pool: pool}
}

func (r *SQLRepository) PostExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1)`, id).Scan(&exists)
	return exists, err
}

func (r *SQLRepository) InsertPostWithTerms(ctx context.Context, p Post, terms map[string]int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO posts (id, url, title, author, published_at, fetched_at)
         VALUES ($1,$2,$3,$4,$5,$6)
         ON CONFLICT (id) DO NOTHING`,
		p.ID, p.URL, p.Title, p.Author, p.PublishedAt, p.FetchedAt,
	)
	if err != nil {
		return err
	}

	if len(terms) > 0 {
		stmt := `INSERT INTO post_terms (post_id, term, term_count)
                 VALUES ($1, $2, $3)
                 ON CONFLICT (post_id, term) DO UPDATE set term_count = EXCLUDED.term_count`
		for term, cnt := range terms {
			if term == "" {
				continue
			}
			if _, err := tx.Exec(ctx, stmt, p.ID, term, cnt); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *SQLRepository) ListAuthors(ctx context.Context, start, end time.Time) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT author FROM posts WHERE published_at > $1 AND published_at < $2 ORDER BY author`,
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *SQLRepository) ListPostsByAuthor(ctx context.Context, author string, start, end time.Time) ([]PostSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT title, published_at FROM posts
         WHERE author = $1 AND published_at > $2 AND published_at < $3
         ORDER BY published_at`,
		author, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		if err := rows.Scan(&p.Title, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SQLRepository) IDF(ctx context.Context, term string, start, end time.Time) (float64, int64, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE published_at > $1 AND published_at < $2`,
		start, end,
	).Scan(&total); err != nil {
		return 0, 0, 0, err
	}
	if total == 0 {
		return 0, 0, 0, errors.New("no documents in the interval")
	}

	var withTerm int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT t.post_id)
         FROM post_terms t
         JOIN posts p ON p.id = t.post_id
         WHERE t.term = $1 AND p.published_at > $2 AND p.published_at < $3`,
		term, start, end,
	).Scan(&withTerm); err != nil {
		return 0, 0, 0, err
	}
	if withTerm == 0 {
		return 0, total, 0, fmt.Errorf("term does not appear in documents")
	}

	idf := math.Log10(float64(total) / float64(withTerm))
	return idf, total, withTerm, nil
}

// IsUniqueViolation tries to detect unique violations in pgx.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
