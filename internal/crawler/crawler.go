package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/martishin/web-crawler/internal/config"
	"github.com/martishin/web-crawler/internal/repository"
	"github.com/martishin/web-crawler/internal/util"
)

type Result struct {
	Processed int64 `json:"processed"`
	Inserted  int64 `json:"inserted"`
	Skipped   int64 `json:"skipped"`
	Older     int64 `json:"older"`
	Errors    int64 `json:"errors"`
}

type Crawler struct {
	cfg    *config.CrawlerConfig
	client *http.Client
	repo   repository.PostsRepository
	loc    *time.Location
}

func New(cfg *config.CrawlerConfig, repo repository.PostsRepository) (*Crawler, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}

	client := &http.Client{Timeout: cfg.ReqTimeout}

	return &Crawler{cfg: cfg, client: client, repo: repo, loc: loc}, nil
}

func (c *Crawler) UpdateToday(ctx context.Context) (*Result, error) {
	now := time.Now().In(c.loc)
	minTime := util.StartOfDay(now, c.loc)
	maxTime := now
	return c.Crawl(ctx, minTime, maxTime)
}

func (c *Crawler) Crawl(ctx context.Context, minTime, maxTime time.Time) (*Result, error) {
	root, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	startURL := root.ResolveReference(&url.URL{Path: c.cfg.IndexPath}).String()

	jobs := make(chan string, 256)
	res := &Result{}

	// Workers
	var wg sync.WaitGroup
	visited := &sync.Map{} // url -> struct{}

	worker := func() {
		defer wg.Done()
		for u := range jobs {
			if ctx.Err() != nil {
				return
			}
			atomic.AddInt64(&res.Processed, 1)
			if err := c.processPost(ctx, u, minTime, maxTime, res); err != nil {
				atomic.AddInt64(&res.Errors, 1)
			}
		}
	}

	for i := 0; i < c.cfg.MaxConcurrency; i++ {
		wg.Add(1)
		go worker()
	}

	// Producer: walk list pages
	pageURL := startURL
	for page := 0; page < c.cfg.MaxPages && pageURL != ""; page++ {
		lp, err := c.fetchListPage(ctx, pageURL)
		if err != nil {
			atomic.AddInt64(&res.Errors, 1)
			break
		}
		for _, postURL := range lp.PostURLs {
			if _, loaded := visited.LoadOrStore(postURL, struct{}{}); loaded {
				continue
			}
			select {
			case jobs <- postURL:
			case <-ctx.Done():
				break
			}
		}
		if lp.NextURL == "" || lp.NextURL == pageURL {
			break
		}
		pageURL = lp.NextURL
		select {
		case <-time.After(c.cfg.PoliteDelay):
		case <-ctx.Done():
			break
		}
	}

	close(jobs)
	wg.Wait()
	return res, nil
}

func (c *Crawler) fetchListPage(ctx context.Context, pageURL string) (*ListPage, error) {
	body, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	lp, err := ParseListPage(body, c.cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	return &lp, nil
}

func (c *Crawler) processPost(ctx context.Context, postURL string, minTime, maxTime time.Time, res *Result) error {
	id := util.MD5Hex(postURL)
	exists, err := c.repo.PostExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		atomic.AddInt64(&res.Skipped, 1)
		return nil
	}

	body, err := c.get(ctx, postURL)
	if err != nil {
		return err
	}
	defer body.Close()

	pp, err := ParsePostPage(body, postURL)
	if err != nil {
		return err
	}

	if pp.PublishedAt.IsZero() {
		return errors.New("could not parse published time")
	}

	if pp.PublishedAt.Before(minTime) || pp.PublishedAt.After(maxTime.Add(24*time.Hour)) {
		atomic.AddInt64(&res.Older, 1)
		return nil
	}

	lang := util.DetectLang(pp.Text)
	terms := util.TermCounts(pp.Text, lang)

	post := repository.Post{
		ID:          id,
		URL:         pp.URL,
		Title:       firstNonEmpty(pp.Title, "(untitled)"),
		Author:      strings.ToLower(strings.TrimSpace(pp.Author)),
		PublishedAt: pp.PublishedAt,
		FetchedAt:   time.Now().UTC(),
	}
	if post.Author == "" {
		post.Author = "unknown"
	}

	if err := c.repo.InsertPostWithTerms(ctx, post, terms); err != nil {
		return err
	}
	atomic.AddInt64(&res.Inserted, 1)
	return nil
}

func (c *Crawler) get(ctx context.Context, u string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return resp.Body, nil
}
