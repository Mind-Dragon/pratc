-- Executor Ledger Schema
-- SQLite-based ledger for crash recovery and exactly-once execution

CREATE TABLE IF NOT EXISTS executor_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    intent_id TEXT NOT NULL,
    transition TEXT NOT NULL,
    preflight_snapshot TEXT NOT NULL,
    mutation_snapshot TEXT,
    timestamp TEXT NOT NULL,
    UNIQUE(intent_id, transition)
);

-- Index for efficient queries by intent_id
CREATE INDEX IF NOT EXISTS idx_executor_ledger_intent_id ON executor_ledger(intent_id);

-- Index for timestamp-based queries (useful for history retrieval)
CREATE INDEX IF NOT EXISTS idx_executor_ledger_timestamp ON executor_ledger(timestamp DESC);
