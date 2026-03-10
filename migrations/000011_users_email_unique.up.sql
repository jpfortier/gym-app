-- Unique email per user. Run after deleting duplicate users.
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email IS NOT NULL;
