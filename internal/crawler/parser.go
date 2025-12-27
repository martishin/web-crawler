package crawler

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
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

// ParseListPage extracts post URLs and the "next" page URL.
func ParseListPage(r io.Reader, baseURL string) (ListPage, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ListPage{}, err
	}

	tok := html.NewTokenizer(r)
	seen := make(map[string]struct{})
	var posts []string
	var next string

	for {
		tt := tok.Next()
		switch tt {
		case html.ErrorToken:
			if tok.Err() == io.EOF {
				return ListPage{PostURLs: posts, NextURL: next}, nil
			}
			return ListPage{}, tok.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tok.TagName()
			if string(name) != "a" || !hasAttr {
				continue
			}
			var href string
			var rel string
			for {
				k, v, more := tok.TagAttr()
				switch string(k) {
				case "href":
					href = string(v)
				case "rel":
					rel = string(v)
				}
				if !more {
					break
				}
			}
			if href == "" {
				continue
			}

			u, err := base.Parse(href)
			if err != nil {
				continue
			}
			u.Fragment = ""

			if next == "" && strings.Contains(rel, "next") {
				next = u.String()
			}

			if u.Host != base.Host {
				continue
			}
			if !articleURLRe.MatchString(u.Path) {
				continue
			}
			if _, ok := seen[u.String()]; ok {
				continue
			}
			seen[u.String()] = struct{}{}
			posts = append(posts, u.String())
		}
	}
}

// ParsePostPage extracts title, author, published time and a best-effort article text.
func ParsePostPage(r io.Reader, pageURL string) (PostPage, error) {
	rawHTML, err := io.ReadAll(r)
	if err != nil {
		return PostPage{}, err
	}

	meta, titleTag, canonical, authorCandidates, timeCandidates, articleText := scanTokens(bytes.NewReader(rawHTML))

	finalURL := canonical
	if finalURL == "" {
		finalURL = pageURL
	}

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

func scanTokens(r io.Reader) (meta map[string]string, titleTag string, canonical string, authorCandidates []string, timeCandidates []string, articleText string) {
	meta = make(map[string]string)
	tok := html.NewTokenizer(r)

	var inTitle bool
	var inArticle bool
	var articleDepth int
	var skipTextDepth int

	var b strings.Builder

	for {
		tt := tok.Next()
		switch tt {
		case html.ErrorToken:
			return meta, strings.TrimSpace(titleTag), canonical, authorCandidates, timeCandidates, b.String()

		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tok.TagName()
			n := string(name)

			if n == "title" {
				inTitle = true
			}
			if n == "article" {
				inArticle = true
				articleDepth = 1
			}

			if n == "script" || n == "style" || n == "noscript" || n == "svg" {
				skipTextDepth++
			}

			if n == "meta" && hasAttr {
				var prop, nameAttr, content string
				for {
					k, v, more := tok.TagAttr()
					switch strings.ToLower(string(k)) {
					case "property":
						prop = string(v)
					case "name":
						nameAttr = string(v)
					case "content":
						content = string(v)
					}
					if !more {
						break
					}
				}
				key := strings.TrimSpace(prop)
				if key == "" {
					key = strings.TrimSpace(nameAttr)
				}
				if key != "" && content != "" {
					meta[strings.ToLower(key)] = strings.TrimSpace(content)
				}
			}

			if n == "link" && hasAttr {
				var rel, href string
				for {
					k, v, more := tok.TagAttr()
					switch strings.ToLower(string(k)) {
					case "rel":
						rel = strings.ToLower(string(v))
					case "href":
						href = string(v)
					}
					if !more {
						break
					}
				}
				if canonical == "" && strings.Contains(rel, "canonical") {
					canonical = strings.TrimSpace(href)
				}
			}

			if (n == "a" || n == "time") && hasAttr {
				var href, datetime string
				for {
					k, v, more := tok.TagAttr()
					switch strings.ToLower(string(k)) {
					case "href":
						href = string(v)
					case "datetime":
						datetime = string(v)
					}
					if !more {
						break
					}
				}
				if href != "" && userURLRe.MatchString(href) {
					authorCandidates = append(authorCandidates, href)
				}
				if datetime != "" {
					timeCandidates = append(timeCandidates, datetime)
				}
			}

			if inArticle && tt == html.StartTagToken && n != "article" {
				articleDepth++
			}

		case html.EndTagToken:
			name, _ := tok.TagName()
			n := string(name)

			if n == "title" {
				inTitle = false
			}
			if n == "article" {
				inArticle = false
				articleDepth = 0
			} else if inArticle {
				articleDepth--
				if articleDepth <= 0 {
					inArticle = false
					articleDepth = 0
				}
			}

			if n == "script" || n == "style" || n == "noscript" || n == "svg" {
				if skipTextDepth > 0 {
					skipTextDepth--
				}
			}

		case html.TextToken:
			text := strings.TrimSpace(string(tok.Text()))
			if text == "" {
				continue
			}
			if inTitle {
				if titleTag == "" {
					titleTag = text
				}
				continue
			}
			if skipTextDepth > 0 {
				continue
			}
			if inArticle {
				b.WriteString(text)
				b.WriteRune(' ')
			}
		}
	}
}

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

func parseTimeBestEffort(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, io.EOF
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05-0700", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}

	if t, err := parseRussianDateTime(s, time.Local); err == nil {
		return t, nil
	}

	return time.Time{}, io.EOF
}

func parseRussianDateTime(s string, loc *time.Location) (time.Time, error) {
	lower := strings.ToLower(strings.TrimSpace(s))
	lower = strings.ReplaceAll(lower, "\u00a0", " ")
	lower = strings.ReplaceAll(lower, "  ", " ")

	now := time.Now().In(loc)

	if strings.HasPrefix(lower, "сегодня") {
		return parseTimeOnly(now, lower)
	}
	if strings.HasPrefix(lower, "вчера") {
		return parseTimeOnly(now.AddDate(0, 0, -1), lower)
	}

	months := map[string]time.Month{
		"янв": time.January, "января": time.January,
		"фев": time.February, "февраля": time.February,
		"мар": time.March, "марта": time.March,
		"апр": time.April, "апреля": time.April,
		"май": time.May, "мая": time.May,
		"июн": time.June, "июня": time.June,
		"июл": time.July, "июля": time.July,
		"авг": time.August, "августа": time.August,
		"сен": time.September, "сентября": time.September,
		"окт": time.October, "октября": time.October,
		"ноя": time.November, "ноября": time.November,
		"дек": time.December, "декабря": time.December,
	}

	fields := strings.Fields(lower)
	if len(fields) < 3 {
		return time.Time{}, io.EOF
	}

	day, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, io.EOF
	}

	month, ok := months[fields[1]]
	if !ok {
		return time.Time{}, io.EOF
	}

	year := now.Year()
	i := 2
	if y, err := strconv.Atoi(fields[2]); err == nil {
		year = y
		i = 3
	}

	var timeStr string
	for ; i < len(fields); i++ {
		if strings.Contains(fields[i], ":") {
			timeStr = fields[i]
			break
		}
	}
	if timeStr == "" {
		return time.Time{}, io.EOF
	}

	hh, mm, err := parseHHMM(timeStr)
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(year, month, day, hh, mm, 0, 0, loc), nil
}

func parseTimeOnly(base time.Time, s string) (time.Time, error) {
	fields := strings.Fields(s)
	for _, f := range fields {
		if strings.Contains(f, ":") {
			hh, mm, err := parseHHMM(f)
			if err != nil {
				return time.Time{}, err
			}
			y, m, d := base.Date()
			return time.Date(y, m, d, hh, mm, 0, 0, base.Location()), nil
		}
	}
	return time.Time{}, io.EOF
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, io.EOF
	}
	hh, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	mm, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hh, mm, nil
}
