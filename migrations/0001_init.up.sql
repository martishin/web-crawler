CREATE TABLE IF NOT EXISTS posts (
    id           TEXT PRIMARY KEY,
    url          TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    author       TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts (published_at);
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts (author);

CREATE TABLE IF NOT EXISTS post_terms (
    post_id    TEXT NOT NULL,
    term       TEXT NOT NULL,
    term_count INT  NOT NULL,
    PRIMARY KEY (post_id, term)
);

CREATE INDEX IF NOT EXISTS idx_post_terms_term ON post_terms (term);
