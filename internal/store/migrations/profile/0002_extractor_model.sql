-- profile/0002_extractor_model.sql
-- Add extractor model to provider_config.

ALTER TABLE provider_config
ADD COLUMN extractor_model TEXT NOT NULL DEFAULT '';
