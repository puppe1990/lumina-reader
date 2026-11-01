-- up
-- Traduções da seleção ficam em cache: palavras e frases repetidas voltam na
-- hora, sem falar de novo com o provedor.
CREATE TABLE IF NOT EXISTS translation_cache (
    fingerprint TEXT PRIMARY KEY,
    traducao TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- down
DROP TABLE IF EXISTS translation_cache;
