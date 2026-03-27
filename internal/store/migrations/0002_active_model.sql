-- 0002_active_model.sql
-- Separate the active model from the provider credential so it can be
-- changed without re-entering the API key.
ALTER TABLE provider_config ADD COLUMN active_model TEXT NOT NULL DEFAULT '';
