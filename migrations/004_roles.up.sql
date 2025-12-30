-- เพิ่ม role ถ้ายังไม่มี (กันพังเวลา deploy)
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS role text;

-- set default + not null
ALTER TABLE users
  ALTER COLUMN role SET DEFAULT 'user';

UPDATE users
SET role = 'user'
WHERE role IS NULL OR trim(role) = '';

ALTER TABLE users
  ALTER COLUMN role SET NOT NULL;

-- จำกัดค่า role
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'users_role_chk'
  ) THEN
    ALTER TABLE users
      ADD CONSTRAINT users_role_chk CHECK (role IN ('admin','user'));
  END IF;
END $$;
