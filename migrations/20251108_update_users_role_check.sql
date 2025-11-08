-- Allow 'admin' role and keep existing roles
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('fan','creator','admin'));

