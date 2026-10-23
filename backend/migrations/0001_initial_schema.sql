CREATE SCHEMA IF NOT EXISTS scheduler;

CREATE TABLE IF NOT EXISTS scheduler.companies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scheduler.users (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES scheduler.companies(id) ON DELETE CASCADE,
    username TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scheduler.events (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES scheduler.companies(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    address TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    recurrence_months INTEGER NOT NULL DEFAULT 0 CHECK (recurrence_months BETWEEN 0 AND 120),
    description TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL REFERENCES scheduler.users(id),
    assigned_to TEXT REFERENCES scheduler.users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS events_company_start_idx ON scheduler.events(company_id, starts_at);
CREATE INDEX IF NOT EXISTS events_company_address_idx ON scheduler.events(company_id, address);
