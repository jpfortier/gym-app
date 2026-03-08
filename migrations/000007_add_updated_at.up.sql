-- Add updated_at to tables. Use trigger to auto-update on UPDATE.
ALTER TABLE users ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE exercise_categories ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE exercise_variants ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE workout_sessions ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE log_entries ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE log_entry_sets ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE personal_records ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE notes ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER exercise_categories_updated_at BEFORE UPDATE ON exercise_categories
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER exercise_variants_updated_at BEFORE UPDATE ON exercise_variants
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER workout_sessions_updated_at BEFORE UPDATE ON workout_sessions
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER log_entries_updated_at BEFORE UPDATE ON log_entries
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER log_entry_sets_updated_at BEFORE UPDATE ON log_entry_sets
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER personal_records_updated_at BEFORE UPDATE ON personal_records
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER notes_updated_at BEFORE UPDATE ON notes
  FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
