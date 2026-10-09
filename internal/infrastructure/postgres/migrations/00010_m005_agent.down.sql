-- +goose Down
DROP TABLE IF EXISTS sandbox_leases;
DROP TABLE IF EXISTS knowledge_candidates;
DROP TABLE IF EXISTS discussion_rounds;
DROP TABLE IF EXISTS discussion_batches;
DROP TABLE IF EXISTS context_checkpoints;
DROP TABLE IF EXISTS run_events;
DROP TABLE IF EXISTS tool_calls;
DROP TABLE IF EXISTS model_calls;
DROP TABLE IF EXISTS agent_runs;
DROP TABLE IF EXISTS agent_sessions;
