package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	dbgen "github.com/martishin/web-crawler/internal/db/generated"
)

type SQLCRepository struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func newSQLCRepository(pool *pgxpool.Pool) *SQLCRepository {
	return &SQLCRepository{
		pool: pool,
		q:    dbgen.New(pool),
	}
}

func (r *SQLCRepository) PostExists(ctx context.Context, id string) (bool, error) {
	return r.q.PostExists(ctx, id)
}

func (r *SQLCRepository) InsertPostWithTerms(ctx context.Context, p Post, terms map[string]int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	if err := qtx.CreatePost(ctx, dbgen.CreatePostParams{
		ID:          p.ID,
		Url:         p.URL,
		Title:       p.Title,
		Author:      p.Author,
		PublishedAt: p.PublishedAt,
		FetchedAt:   p.FetchedAt,
	}); err != nil {
		return err
	}

	for term, cnt := range terms {
		if term == "" || cnt <= 0 {
			continue
		}
		if err := qtx.UpsertPostTerm(ctx, dbgen.UpsertPostTermParams{
			PostID:    p.ID,
			Term:      term,
			TermCount: int32(cnt),
		}); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *SQLCRepository) ListAuthors(ctx context.Context, start, end time.Time) ([]string, error) {
	return r.q.ListAuthorsByTime(ctx, dbgen.ListAuthorsByTimeParams{
		PublishedAt:   start,
		PublishedAt_2: end,
	})
}

func (r *SQLCRepository) ListPostsByAuthor(ctx context.Context, author string, start, end time.Time) ([]PostSummary, error) {
	rows, err := r.q.ListPostsByAuthorAndTime(ctx, dbgen.ListPostsByAuthorAndTimeParams{
		Author:        author,
		PublishedAt:   start,
		PublishedAt_2: end,
	})
	if err != nil {
		return nil, err
	}

	out := make([]PostSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, PostSummary{
			Title:       row.Title,
			PublishedAt: row.PublishedAt,
		})
	}
	return out, nil
}

func (r *SQLCRepository) IDF(ctx context.Context, term string, start, end time.Time) (float64, int64, int64, error) {
	total, err := r.q.CountPostsByTime(ctx, dbgen.CountPostsByTimeParams{
		PublishedAt:   start,
		PublishedAt_2: end,
	})
	if err != nil {
		return 0, 0, 0, err
	}
	if total == 0 {
		return 0, 0, 0, errors.New("no documents in the interval")
	}

	withTerm, err := r.q.CountPostsContainingTermByTime(ctx, dbgen.CountPostsContainingTermByTimeParams{
		Term:          term,
		PublishedAt:   start,
		PublishedAt_2: end,
	})
	if err != nil {
		return 0, 0, 0, err
	}
	if withTerm == 0 {
		return 0, total, 0, fmt.Errorf("term does not appear in documents")
	}

	return math.Log10(float64(total) / float64(withTerm)), total, withTerm, nil
}
