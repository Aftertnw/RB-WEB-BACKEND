ALTER TABLE users ADD COLUMN IF NOT EXISTS is_approved BOOLEAN DEFAULT FALSE;

-- Update existing users to be approved so they are not locked out
UPDATE users SET is_approved = TRUE WHERE is_approved IS FALSE;
