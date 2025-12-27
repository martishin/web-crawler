-- name: CreatePost :exec
INSERT INTO posts (id, url, title, author, published_at, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (id) DO NOTHING;

-- name: PostExists :one
SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1) AS exists;

-- name: ListAuthorsByTime :many
SELECT DISTINCT author
FROM posts
WHERE published_at > $1 AND published_at < $2
ORDER BY author;

-- name: ListPostsByAuthorAndTime :many
SELECT title, published_at
FROM posts
WHERE author = $1 AND published_at > $2 AND published_at < $3
ORDER BY published_at;

-- name: CountPostsByTime :one
SELECT COUNT(*) AS count
FROM posts
WHERE published_at > $1 AND published_at < $2;

-- name: CountPostsContainingTermByTime :one
SELECT COUNT(DISTINCT p.id) AS count
FROM posts p
    JOIN post_terms t ON t.post_id = p.id
WHERE t.term = $1 AND p.published_at > $2 AND p.published_at < $3;

-- name: UpsertPostTerm :exec
INSERT INTO post_terms (post_id, term, term_count)
VALUES ($1, $2, $3)
    ON CONFLICT (post_id, term)
DO UPDATE SET term_count = EXCLUDED.term_count;
