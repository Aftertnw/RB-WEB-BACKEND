CREATE TABLE IF NOT EXISTS judgment_doc_counters (
  year int PRIMARY KEY,
  last_no int NOT NULL
);

CREATE OR REPLACE FUNCTION next_judgment_doc_no() RETURNS text AS $$
DECLARE
  y int := EXTRACT(YEAR FROM now());
  n int;
BEGIN
  LOOP
    UPDATE judgment_doc_counters
    SET last_no = last_no + 1
    WHERE year = y
    RETURNING last_no INTO n;

    IF FOUND THEN
      EXIT;
    END IF;

    BEGIN
      INSERT INTO judgment_doc_counters(year, last_no) VALUES (y, 0);
    EXCEPTION WHEN unique_violation THEN
    END;
  END LOOP;

  RETURN 'JG-' || y::text || '-' || LPAD(n::text, 4, '0');
END;
$$ LANGUAGE plpgsql;

ALTER TABLE judgments
ADD COLUMN IF NOT EXISTS doc_no text UNIQUE;

UPDATE judgments
SET doc_no = next_judgment_doc_no()
WHERE doc_no IS NULL;
