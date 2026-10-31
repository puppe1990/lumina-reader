-- up
-- As capas marcadas como "missing" até aqui foram gravadas quando o poppler
-- ainda não estava instalado no servidor (PDF) — re-tenta na próxima request.
DELETE FROM book_covers WHERE missing = 1;

-- down
-- (limpeza de dados; nada a reverter)
