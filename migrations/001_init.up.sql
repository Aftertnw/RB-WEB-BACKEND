CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS judgments (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  title text NOT NULL,
  case_no text NULL,
  court text NULL,
  judgment_date date NULL,
  parties text NULL,
  facts text NULL,
  issues text NULL,
  holding text NULL,
  notes text NULL,
  tags text[] NOT NULL DEFAULT ARRAY[]::text[],
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_judgments_date ON judgments (judgment_date DESC);
CREATE INDEX IF NOT EXISTS idx_judgments_updated ON judgments (updated_at DESC);
