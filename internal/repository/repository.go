package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostsRepository interface {
	PostExists(ctx context.Context, id string) (bool, error)
	InsertPostWithTerms(ctx context.Context, p Post, terms map[string]int) error

	ListAuthors(ctx context.Context, start, end time.Time) ([]string, error)
	ListPostsByAuthor(ctx context.Context, author string, start, end time.Time) ([]PostSummary, error)

	IDF(ctx context.Context, term string, start, end time.Time) (idf float64, totalDocs int64, docsWithTerm int64, err error)
}

type Post struct {
	ID          string
	URL         string
	Title       string
	Author      string
	PublishedAt time.Time
	FetchedAt   time.Time
}

type PostSummary struct {
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
}

var ErrWordNotFound = errors.New("word not found")

func NewRepository(pool *pgxpool.Pool) PostsRepository {
	return newSQLCRepository(pool)
}
