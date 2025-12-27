package crawler

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	dps "github.com/markusmobius/go-dateparser"
)

type ListPage struct {
	PostURLs []string
	NextURL  string
}

type PostPage struct {
	URL         string
	Title       string
	Author      string
	PublishedAt time.Time
	Text        string
}

var (
	articleURLRe = regexp.MustCompile(`^/(?:ru|en)/articles/\d+/`)
	userURLRe    = regexp.MustCompile(`^/(?:ru|en)/users/[^/]+/`)
)

// ParseListPage extracts post URLs and the "next" page URL using goquery.
func ParseListPage(r io.Reader, baseURL string) (ListPage, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ListPage{}, err
	}

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return ListPage{}, err
	}

	seen := make(map[string]struct{})
	var posts []string
	var next string

	// Next page link: <a rel="next" href="...">
	if sel := doc.Find(`a[rel="next"]`).First(); sel != nil {
		if href, ok := sel.Attr("href"); ok && strings.TrimSpace(href) != "" {
			if u, err := base.Parse(href); err == nil {
				u.Fragment = ""
				next = u.String()
			}
		}
	}

	// Collect post URLs by URL pattern (not relying on fragile CSS classes).
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			return
		}
		u, err := base.Parse(href)
		if err != nil {
			return
		}
		u.Fragment = ""
		if u.Host != base.Host {
			return
		}
		if !articleURLRe.MatchString(u.Path) {
			return
		}
		abs := u.String()
		if _, exists := seen[abs]; exists {
			return
		}
		seen[abs] = struct{}{}
		posts = append(posts, abs)
	})

	return ListPage{PostURLs: posts, NextURL: next}, nil
}

// ParsePostPage extracts title, author, published time and article text using goquery.
func ParsePostPage(r io.Reader, pageURL string) (PostPage, error) {
	rawHTML, err := io.ReadAll(r)
	if err != nil {
		return PostPage{}, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(rawHTML))
	if err != nil {
		return PostPage{}, err
	}

	// --- Meta tags ---
	meta := make(map[string]string)
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		var key string
		if v, ok := s.Attr("property"); ok && strings.TrimSpace(v) != "" {
			key = strings.ToLower(strings.TrimSpace(v))
		} else if v, ok := s.Attr("name"); ok && strings.TrimSpace(v) != "" {
			key = strings.ToLower(strings.TrimSpace(v))
		}
		if key == "" {
			return
		}
		if content, ok := s.Attr("content"); ok && strings.TrimSpace(content) != "" {
			meta[key] = strings.TrimSpace(content)
		}
	})

	titleTag := strings.TrimSpace(doc.Find("title").First().Text())

	// Canonical URL
	canonical := ""
	if sel := doc.Find(`link[rel="canonical"]`).First(); sel != nil {
		if href, ok := sel.Attr("href"); ok {
			canonical = strings.TrimSpace(href)
		}
	}
	finalURL := canonical
	if finalURL == "" {
		finalURL = pageURL
	}

	// Author candidates: user profile URLs.
	var authorCandidates []string
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok && userURLRe.MatchString(href) {
			authorCandidates = append(authorCandidates, href)
		}
	})

	// Time candidates: <time datetime="..."> or text content.
	var timeCandidates []string
	doc.Find("time").Each(func(_ int, s *goquery.Selection) {
		if dt, ok := s.Attr("datetime"); ok && strings.TrimSpace(dt) != "" {
			timeCandidates = append(timeCandidates, strings.TrimSpace(dt))
		} else {
			t := strings.TrimSpace(s.Text())
			if t != "" {
				timeCandidates = append(timeCandidates, t)
			}
		}
	})

	// Article text: extract block-like nodes and join with separators (prevents "word glue").
	var b strings.Builder
	doc.Find("article").
		Find("p, li, h1, h2, h3, blockquote").
		Each(func(_ int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if t == "" {
				return
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(t)
		})
	articleText := b.String()

	title := firstNonEmpty(
		meta["og:title"],
		meta["twitter:title"],
		titleTag,
	)

	author := firstNonEmpty(
		meta["article:author"],
		bestUserFromCandidates(authorCandidates),
	)
	author = strings.ToLower(strings.TrimSpace(author))

	publishedAtStr := firstNonEmpty(
		meta["article:published_time"],
		meta["og:published_time"],
		meta["publish_date"],
		firstNonEmpty(timeCandidates...),
	)
	publishedAt, _ := parseTimeBestEffort(publishedAtStr)

	text := strings.TrimSpace(articleText)

	return PostPage{
		URL:         finalURL,
		Title:       title,
		Author:      author,
		PublishedAt: publishedAt,
		Text:        text,
	}, nil
}

// Reuse your existing helper from helpers.go.
func bestUserFromCandidates(hrefs []string) string {
	best := ""
	for _, h := range hrefs {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if best == "" || len(h) < len(best) {
			best = h
		}
	}
	if best == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(best, "/"), "/")
	if len(parts) >= 3 {
		return parts[len(parts)-1]
	}
	return best
}

// parseTimeBestEffort uses RFC3339 first, then go-dateparser which supports Russian
func parseTimeBestEffort(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time string")
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	dt, err := dps.Parse(nil, s)
	if err != nil {
		return time.Time{}, err
	}

	return dt.Time, nil
}
