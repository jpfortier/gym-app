DROP TRIGGER IF EXISTS users_updated_at ON users;
DROP TRIGGER IF EXISTS exercise_categories_updated_at ON exercise_categories;
DROP TRIGGER IF EXISTS exercise_variants_updated_at ON exercise_variants;
DROP TRIGGER IF EXISTS workout_sessions_updated_at ON workout_sessions;
DROP TRIGGER IF EXISTS log_entries_updated_at ON log_entries;
DROP TRIGGER IF EXISTS log_entry_sets_updated_at ON log_entry_sets;
DROP TRIGGER IF EXISTS personal_records_updated_at ON personal_records;
DROP TRIGGER IF EXISTS notes_updated_at ON notes;
DROP FUNCTION IF EXISTS set_updated_at();

ALTER TABLE users DROP COLUMN updated_at;
ALTER TABLE exercise_categories DROP COLUMN updated_at;
ALTER TABLE exercise_variants DROP COLUMN updated_at;
ALTER TABLE workout_sessions DROP COLUMN updated_at;
ALTER TABLE log_entries DROP COLUMN updated_at;
ALTER TABLE log_entry_sets DROP COLUMN updated_at;
ALTER TABLE personal_records DROP COLUMN updated_at;
ALTER TABLE notes DROP COLUMN updated_at;
