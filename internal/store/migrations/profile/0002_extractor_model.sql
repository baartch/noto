-- profile/0002_extractor_model.sql
-- Add extractor + embeddings models to provider_config.

ALTER TABLE provider_config
ADD COLUMN extractor_model TEXT NOT NULL DEFAULT '';

ALTER TABLE provider_config
ADD COLUMN embeddings_model TEXT NOT NULL DEFAULT '';
