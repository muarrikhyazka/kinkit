-- Kinkit Postgres schema.
--
-- HOW THIS FILE EVOLVES (read before editing)
-- This whole file is embedded in the binary (schema.go) and executed on
-- EVERY boot against databases that may already hold production data, so
-- every statement MUST be idempotent and additive:
--   * New table:   CREATE TABLE IF NOT EXISTS foo (...);
--   * New column:  ALTER TABLE foo ADD COLUMN IF NOT EXISTS bar TYPE [DEFAULT ...];
--                  (do NOT edit an existing CREATE TABLE to add a column - that
--                  only affects fresh installs; existing DBs need the ALTER.
--                  New NOT NULL columns need a DEFAULT so old rows are valid.)
--   * New index:   CREATE [UNIQUE] INDEX IF NOT EXISTS foo_bar_idx ON foo(bar);
--   * Never DROP/RENAME/change a type here; do destructive changes as a
--     deliberate one-off migration instead.
-- Append new statements at the end of the file (or under the table they
-- belong to), with a short comment saying what they are for.
-- Dates/times are UTC: use TIMESTAMPTZ for instants and DATE for UTC
-- calendar dates (the app pool sets timezone=UTC on every connection).
--
-- Ported from the SQLite original.
-- Every data row carries account_id (the owning user's id) so each signup
-- gets fully isolated data. ponytail: "account" == single user for now,
-- no shared/family multi-user accounts yet — add a separate accounts/members
-- table if that's ever needed.
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,            -- NULL if signed up via Google only
    google_sub TEXT UNIQUE,        -- Google account id, NULL if password-only
    display_name TEXT,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,     -- super-admin: can view/manage all accounts
    is_disabled BOOLEAN NOT NULL DEFAULT FALSE,  -- admin-controlled account suspension
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,  -- set once /api/verify-email consumes the token
    verification_token TEXT,                     -- NULL once verified
    currency TEXT NOT NULL DEFAULT 'IDR',
    locale TEXT NOT NULL DEFAULT 'id-ID',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Password-reset tokens, one-time use, 1hr expiry.
CREATE TABLE IF NOT EXISTS password_resets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- "bank"/"asset" accounts the user tags transactions/assets with
CREATE TABLE IF NOT EXISTS accounts (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'bank',  -- bank | asset
    UNIQUE(account_id, name)
);

CREATE TABLE IF NOT EXISTS categories (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    main_category TEXT NOT NULL CHECK(main_category IN ('Needs','Wants','Savings')),
    UNIQUE(account_id, name)
);

CREATE TABLE IF NOT EXISTS income_sources (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    UNIQUE(account_id, name)
);

CREATE TABLE IF NOT EXISTS budget_income (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    source_id BIGINT NOT NULL REFERENCES income_sources(id),
    amount BIGINT NOT NULL DEFAULT 0,
    UNIQUE(account_id, year, month, source_id)
);

CREATE TABLE IF NOT EXISTS budget_expense (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    category_id BIGINT NOT NULL REFERENCES categories(id),
    amount BIGINT NOT NULL DEFAULT 0,
    UNIQUE(account_id, year, month, category_id)
);

CREATE TABLE IF NOT EXISTS spending (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    tx_date DATE NOT NULL,
    description TEXT NOT NULL,
    category_id BIGINT REFERENCES categories(id),
    category_name TEXT,
    amount BIGINT NOT NULL,
    checked BOOLEAN NOT NULL DEFAULT FALSE,
    source TEXT NOT NULL DEFAULT 'web',  -- web | telegram | discord | migration
    client_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, client_id)
);

CREATE TABLE IF NOT EXISTS assets (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    description TEXT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    account_ref BIGINT REFERENCES accounts(id),
    kind TEXT NOT NULL DEFAULT 'liquid'  -- liquid | non_liquid
);

CREATE TABLE IF NOT EXISTS asset_target (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    target_value BIGINT NOT NULL,
    last_update TEXT
);

CREATE TABLE IF NOT EXISTS savings_list (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    name TEXT NOT NULL,
    amount BIGINT NOT NULL DEFAULT 0,
    UNIQUE(account_id, year, month, name)
);

-- Chat-platform integrations (Telegram, Discord, ...), one row per platform
-- per account. bot_token is this account's secret used by the platform
-- adapter process to authenticate its /api/bot/* calls.
CREATE TABLE IF NOT EXISTS integrations (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    platform TEXT NOT NULL CHECK(platform IN ('telegram','discord')),
    bot_token TEXT NOT NULL UNIQUE,
    platform_token TEXT,
    chat_id TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, platform)
);

CREATE TABLE IF NOT EXISTS reminder_schedules (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    job_name TEXT NOT NULL,
    days TEXT NOT NULL,
    hour INTEGER NOT NULL,
    minute INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run TEXT,
    UNIQUE(account_id, job_name)
);

-- Single-row key/value store for cross-process signals (e.g. bumping
-- integrations_version on every integrations write lets the bot
-- multiplexer processes detect a change without re-fetching everything).
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS mandatory_ingredients (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    UNIQUE(account_id, name)
);

CREATE TABLE IF NOT EXISTS category_keywords (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id),
    keyword TEXT NOT NULL,
    category TEXT NOT NULL,
    UNIQUE(account_id, keyword)
);

-- Gamification: one row per account. Points earned by logging spending
-- (kept simple - no separate ledger table since we only need running
-- totals + a daily streak, not a full history of point events).
CREATE TABLE IF NOT EXISTS user_points (
    account_id BIGINT PRIMARY KEY REFERENCES users(id),
    points BIGINT NOT NULL DEFAULT 0,
    streak_days INTEGER NOT NULL DEFAULT 0,
    last_log_date DATE
);

-- ---------------------------------------------------------------------------
-- Additive migrations (ALTER TABLE ... ADD COLUMN IF NOT EXISTS, new indexes,
-- new tables) go below this line.
-- ---------------------------------------------------------------------------

-- === family (households, members, roles, per-member gamification) ===
-- "account" (users row) stays the household; every finance row keeps
-- account_id. members are the people who log in to a household.
ALTER TABLE users ADD COLUMN IF NOT EXISTS household_name TEXT;

CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    display_name TEXT,
    role TEXT NOT NULL DEFAULT 'kid' CHECK (role IN ('owner','parent','kid','viewer')),
    allowance_monthly NUMERIC,          -- kids: monthly allowance (stored currency, IDR)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS members_account_idx ON members(account_id);
-- exactly one owner per household
CREATE UNIQUE INDEX IF NOT EXISTS members_one_owner_idx ON members(account_id) WHERE role = 'owner';

-- Boot migration: every pre-family account gets an owner member carrying
-- the account's own email + password hash (legacy werkzeug hashes included;
-- they're upgraded to bcrypt on first login as before).
INSERT INTO members (account_id, email, password_hash, display_name, role, created_at)
SELECT u.id, u.email, u.password_hash, u.display_name, 'owner', u.created_at
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM members m WHERE m.account_id = u.id AND m.role = 'owner')
  AND NOT EXISTS (SELECT 1 FROM members m WHERE m.email = u.email);

-- who logged a spending row (NULL: bots, imports, pre-family rows)
ALTER TABLE spending ADD COLUMN IF NOT EXISTS member_id BIGINT REFERENCES members(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS spending_member_date_idx ON spending(member_id, tx_date);

-- password resets now target a member (NULL on legacy rows = account owner)
ALTER TABLE password_resets ADD COLUMN IF NOT EXISTS member_id BIGINT REFERENCES members(id) ON DELETE CASCADE;

-- per-member running points + streak (user_points stays the household total)
CREATE TABLE IF NOT EXISTS member_points (
    member_id BIGINT PRIMARY KEY REFERENCES members(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    points BIGINT NOT NULL DEFAULT 0,
    streak_days INTEGER NOT NULL DEFAULT 0,
    best_streak INTEGER NOT NULL DEFAULT 0,
    last_log_date DATE
);

-- points ledger, for "points this month" leaderboards. member_id NULL =
-- household-level award (e.g. a family challenge bonus).
CREATE TABLE IF NOT EXISTS point_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_id BIGINT REFERENCES members(id) ON DELETE CASCADE,
    points BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS point_events_account_created_idx ON point_events(account_id, created_at);

-- habit badges earned by a member (evaluated lazily on GET /api/gamification)
CREATE TABLE IF NOT EXISTS member_badges (
    id BIGSERIAL PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    badge_key TEXT NOT NULL,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(member_id, badge_key)
);

-- household weekly challenge completions; the UNIQUE makes the bonus a
-- one-time award per challenge per week (period_start = Monday, UTC).
CREATE TABLE IF NOT EXISTS challenge_completions (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge_key TEXT NOT NULL,
    period_start DATE NOT NULL,
    bonus BIGINT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, challenge_key, period_start)
);
-- === end family ===

-- === data === (import/export, receipts, dashboard layouts, TV share links)
-- receipt photo for a spending row: path relative to DATA_DIR/receipts
ALTER TABLE spending ADD COLUMN IF NOT EXISTS receipt_path TEXT;
-- CSV import batch id, so a whole import can be undone in one go
ALTER TABLE spending ADD COLUMN IF NOT EXISTS import_batch TEXT;
CREATE INDEX IF NOT EXISTS spending_import_batch_idx ON spending(account_id, import_batch) WHERE import_batch IS NOT NULL;

-- Custom dashboard widget layout, synced across devices. member_id NULL =
-- the account-wide layout (per-member layouts can use the same table).
CREATE TABLE IF NOT EXISTS dashboard_layouts (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_id BIGINT,
    layout JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS dashboard_layouts_owner_idx ON dashboard_layouts(account_id, (COALESCE(member_id, 0)));

-- Read-only share links for the TV / car display (/tv?k=<token>).
CREATE TABLE IF NOT EXISTS tv_tokens (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS tv_tokens_account_idx ON tv_tokens(account_id);
-- === end data ===

-- === money ===
-- Recurring transaction rules; the money worker inserts spending rows
-- (source='recurring', client_id='recurring:<id>:<YYYY-MM-DD>') for every
-- due occurrence up to today (UTC). day_of_month for monthly/yearly (yearly
-- uses start_date's month), weekday 0=Sunday..6 for weekly.
CREATE TABLE IF NOT EXISTS recurring_transactions (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    amount BIGINT NOT NULL,
    frequency TEXT NOT NULL CHECK(frequency IN ('monthly','weekly','yearly')),
    day_of_month INTEGER CHECK(day_of_month BETWEEN 1 AND 31),
    weekday INTEGER CHECK(weekday BETWEEN 0 AND 6),
    start_date DATE NOT NULL,
    end_date DATE,
    last_generated_date DATE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS recurring_transactions_account_idx ON recurring_transactions(account_id);

-- Actual income received (budget_income is the plan).
CREATE TABLE IF NOT EXISTS income_entries (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id BIGINT REFERENCES income_sources(id) ON DELETE SET NULL,
    tx_date DATE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    amount BIGINT NOT NULL,
    client_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, client_id)
);
CREATE INDEX IF NOT EXISTS income_entries_account_date_idx ON income_entries(account_id, tx_date);

-- Savings goals. Progress = saved_amount + spending in category_id since
-- created_at + current value of asset_ids.
CREATE TABLE IF NOT EXISTS savings_goals (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    target_amount BIGINT NOT NULL,
    target_date DATE,
    saved_amount BIGINT NOT NULL DEFAULT 0,
    category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    asset_ids BIGINT[] NOT NULL DEFAULT '{}',
    emoji TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS savings_goals_account_idx ON savings_goals(account_id);

-- Budget alerts: per-account opt-out (account == users row) and the
-- idempotency log, one row per (category, month, threshold 80|100).
ALTER TABLE users ADD COLUMN IF NOT EXISTS alerts_enabled BOOLEAN NOT NULL DEFAULT TRUE;
CREATE TABLE IF NOT EXISTS budget_alerts_sent (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    threshold INTEGER NOT NULL,
    spent BIGINT NOT NULL DEFAULT 0,
    budget BIGINT NOT NULL DEFAULT 0,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, category_id, year, month, threshold)
);
-- === end money ===
