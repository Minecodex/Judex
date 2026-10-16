-- Read-only cache observation report.
-- Temporary normalized fields; only the latest rolling 24 hours are visible.
-- Execute as an authorized test/admin actor and narrow project/run/time scope.
WITH calls AS (
  SELECT
    project_id, run_id, state,
    COALESCE(usage_json->'observation'->>'modelName', 'unknown') AS model_name,
    usage_known,
    (usage_json->'observation'->>'inputTokens')::bigint AS input_tokens,
    (usage_json->'observation'->>'cacheReadTokens')::bigint AS cache_read_tokens,
    EXTRACT(EPOCH FROM ended_at - started_at) * 1000 AS elapsed_ms
  FROM model_calls
  WHERE ended_at > now()-interval '24 hours' AND usage_json ? 'observation'
  -- Add the owned project's ID, run ID and time range for acceptance export.
), classified AS (
  SELECT *,
    usage_known AND input_tokens IS NOT NULL
    AND cache_read_tokens IS NOT NULL
    AND input_tokens >= 0 AND cache_read_tokens >= 0
    AND cache_read_tokens <= input_tokens AS cache_known
  FROM calls
)
SELECT project_id, run_id, model_name,
  count(*) AS completed_calls,
  count(*) FILTER (WHERE cache_known IS TRUE) AS known_cache_calls,
  count(*) FILTER (WHERE cache_known IS NOT TRUE) AS unknown_cache_calls,
  sum(input_tokens) FILTER (WHERE cache_known IS TRUE) AS known_input_tokens,
  sum(cache_read_tokens) FILTER (WHERE cache_known IS TRUE) AS cache_read_tokens,
  round(
    100.0 * sum(cache_read_tokens) FILTER (WHERE cache_known IS TRUE)
    / NULLIF(sum(input_tokens) FILTER (WHERE cache_known IS TRUE), 0),
    2
  ) AS cache_read_percent,
  round(avg(elapsed_ms)::numeric, 1) AS average_call_elapsed_ms
FROM classified
GROUP BY project_id, run_id, model_name
ORDER BY project_id, run_id, model_name;
