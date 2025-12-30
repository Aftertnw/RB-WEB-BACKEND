ALTER TABLE judgments DROP COLUMN IF EXISTS doc_no;
DROP FUNCTION IF EXISTS next_judgment_doc_no();
DROP TABLE IF EXISTS judgment_doc_counters;
