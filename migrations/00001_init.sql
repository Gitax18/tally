-- +goose Up

-- One row per tracked website. The id appears in the script URL (/s/{id}.js).
CREATE TABLE sites (
    id            text        PRIMARY KEY,
    name          text        NOT NULL,
    domain        text        NOT NULL,
    timezone      text        NOT NULL DEFAULT 'UTC',
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_event_at timestamptz,

    CONSTRAINT sites_id_check     CHECK (id ~ '^[a-z0-9]{10}$'),
    CONSTRAINT sites_name_check   CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT sites_domain_check CHECK (char_length(domain) BETWEEN 1 AND 253)
);

-- A random salt per UTC day. Visitor IDs are hashed with it; salts older than yesterday are
-- deleted, so an old visitor ID can no longer be linked to an address.
CREATE TABLE salts (
    day  date  PRIMARY KEY,
    salt bytea NOT NULL,

    CONSTRAINT salts_salt_check CHECK (length(salt) = 32)
);

-- One row per visit. The dimension columns are the visit's attributes, taken from its first
-- pageview. They are copied onto every event so any breakdown is a single-table WHERE.
CREATE TABLE sessions (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id      text        NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    visitor_id   bigint      NOT NULL,
    started_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    pageviews    integer     NOT NULL DEFAULT 0,
    engaged_ms   bigint      NOT NULL DEFAULT 0,
    entry_path   text        NOT NULL,
    exit_path    text        NOT NULL,

    hostname     text        NOT NULL DEFAULT '',
    source       text        NOT NULL DEFAULT '',
    referrer     text        NOT NULL DEFAULT '',
    utm_source   text        NOT NULL DEFAULT '',
    utm_medium   text        NOT NULL DEFAULT '',
    utm_campaign text        NOT NULL DEFAULT '',
    country      text        NOT NULL DEFAULT '',
    browser      text        NOT NULL DEFAULT '',
    os           text        NOT NULL DEFAULT '',
    device       text        NOT NULL DEFAULT ''
);

CREATE INDEX sessions_site_started_idx ON sessions (site_id, started_at);
CREATE INDEX sessions_site_visitor_seen_idx ON sessions (site_id, visitor_id, last_seen_at DESC);

-- One row per pageview. `name` leaves room for custom events without a migration.
-- session_id has no foreign key: retention deletes both tables in batches and the ingest
-- worker is the only writer.
CREATE TABLE events (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id      text        NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    session_id   bigint      NOT NULL,
    visitor_id   bigint      NOT NULL,
    ts           timestamptz NOT NULL,
    name         text        NOT NULL,
    path         text        NOT NULL,
    engaged_ms   bigint      NOT NULL DEFAULT 0,
    props        jsonb,

    hostname     text        NOT NULL DEFAULT '',
    source       text        NOT NULL DEFAULT '',
    referrer     text        NOT NULL DEFAULT '',
    utm_source   text        NOT NULL DEFAULT '',
    utm_medium   text        NOT NULL DEFAULT '',
    utm_campaign text        NOT NULL DEFAULT '',
    country      text        NOT NULL DEFAULT '',
    browser      text        NOT NULL DEFAULT '',
    os           text        NOT NULL DEFAULT '',
    device       text        NOT NULL DEFAULT ''
);

CREATE INDEX events_site_ts_idx ON events (site_id, ts);

-- +goose Down
DROP TABLE events;
DROP TABLE sessions;
DROP TABLE salts;
DROP TABLE sites;
