ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
-- ไม่แนะนำให้ drop column ทิ้งในระบบจริง แต่ใส่ไว้ให้ครบ
ALTER TABLE users DROP COLUMN IF EXISTS role;
