CREATE TABLE IF NOT EXISTS feature_flags (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL UNIQUE,
    enabled      BOOLEAN NOT NULL DEFAULT true,
    rollout_pct  INTEGER NOT NULL DEFAULT 100,
    regions      TEXT[] NOT NULL DEFAULT '{}',
    description  TEXT,
    updated_by   TEXT,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- seed essential kill-switch flags
INSERT INTO feature_flags (name, enabled, description) VALUES
    ('dispatch.enabled', true, 'Kill-switch: disables matching-service dispatch'),
    ('payments.enabled', true, 'Kill-switch: disables PSP charging'),
    ('surge.enabled',    true, 'Kill-switch: forces surge multiplier to 1.0')
ON CONFLICT (name) DO NOTHING;
