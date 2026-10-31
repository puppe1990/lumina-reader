-- up
-- PDFs usam a página como "capítulo" no progresso; guardar o total de páginas
-- permite percentual e o rótulo "Página X de Y" sem baixar o arquivo.
ALTER TABLE books ADD COLUMN pdf_pages INTEGER NOT NULL DEFAULT 0;

-- down
ALTER TABLE books DROP COLUMN pdf_pages;
