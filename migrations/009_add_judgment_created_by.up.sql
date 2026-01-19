ALTER TABLE judgments ADD COLUMN created_by uuid REFERENCES users(id);
