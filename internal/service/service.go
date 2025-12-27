package service

import (
	"context"
	"strings"
	"time"

	"github.com/martishin/web-crawler/internal/crawler"
	"github.com/martishin/web-crawler/internal/repository"
	"github.com/martishin/web-crawler/internal/util"
)

type Service struct {
	repo    repository.PostsRepository
	crawler *crawler.Crawler
}

func New(repo repository.PostsRepository, crawler *crawler.Crawler) *Service {
	return &Service{repo: repo, crawler: crawler}
}

func (s *Service) UpdateToday(ctx context.Context) (*crawler.Result, error) {
	return s.crawler.UpdateToday(ctx)
}

func (s *Service) ListAuthors(ctx context.Context, start, end time.Time) ([]string, error) {
	return s.repo.ListAuthors(ctx, start, end)
}

func (s *Service) ListPostsByAuthor(ctx context.Context, author string, start, end time.Time) ([]repository.PostSummary, error) {
	author = strings.ToLower(strings.TrimSpace(author))
	return s.repo.ListPostsByAuthor(ctx, author, start, end)
}

func (s *Service) IDF(ctx context.Context, word string, start, end time.Time) (float64, int64, int64, error) {
	word = strings.TrimSpace(strings.ToLower(word))
	if word == "" {
		return 0, 0, 0, repository.ErrWordNotFound
	}

	lang := util.DetectLang(word)

	var term string
	if lang == "russian" {
		term = util.NormalizeRussian(word)
	} else {
		term = util.StemOrIdentity(word, lang)
	}
	if term == "" {
		term = word
	}

	return s.repo.IDF(ctx, term, start, end)
}
