CREATE TABLE voice_settings (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    provider  TEXT NOT NULL DEFAULT '' CHECK (provider IN ('', 'local', 'provider')),
    model     TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO voice_settings (singleton, provider, model) VALUES (1, '', '');
