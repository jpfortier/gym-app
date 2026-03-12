-- Consolidated schema (current state). Reflects migrations.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id text UNIQUE NOT NULL,
    email text,
    name text,
    photo_url text,
    role text NOT NULL DEFAULT 'user',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exercise_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    show_weight boolean NOT NULL DEFAULT true,
    show_reps boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    embedding vector(1536),
    UNIQUE (user_id, name)
);
CREATE UNIQUE INDEX idx_exercise_categories_global_name ON exercise_categories (name) WHERE user_id IS NULL;

CREATE TABLE exercise_variants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id uuid NOT NULL REFERENCES exercise_categories(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    standard boolean NOT NULL DEFAULT false,
    visual_cues text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    embedding vector(1536),
    UNIQUE (category_id, user_id, name)
);
CREATE UNIQUE INDEX idx_exercise_variants_one_standard_per_category
  ON exercise_variants (category_id, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid))
  WHERE standard = true;

CREATE TABLE workout_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, date)
);
CREATE INDEX idx_workout_sessions_user_date ON workout_sessions(user_id, date);

CREATE TABLE log_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE RESTRICT,
    raw_speech text,
    notes text,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_log_entries_session ON log_entries(session_id);
CREATE INDEX idx_log_entries_exercise_created ON log_entries(exercise_variant_id, created_at);

CREATE TABLE log_entry_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    log_entry_id uuid NOT NULL REFERENCES log_entries(id) ON DELETE CASCADE,
    weight decimal,
    reps int NOT NULL,
    set_order int NOT NULL,
    set_type text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_log_entry_sets_log_entry ON log_entry_sets(log_entry_id);

CREATE TABLE notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_category_id uuid REFERENCES exercise_categories(id) ON DELETE CASCADE,
    exercise_variant_id uuid REFERENCES exercise_variants(id) ON DELETE CASCADE,
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE personal_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE RESTRICT,
    pr_type text NOT NULL,
    weight decimal NOT NULL,
    reps int,
    log_entry_set_id uuid REFERENCES log_entry_sets(id) ON DELETE SET NULL,
    image_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_personal_records_user_exercise ON personal_records(user_id, exercise_variant_id, pr_type);

CREATE TABLE ai_usage (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    model text NOT NULL,
    prompt_tokens int NOT NULL,
    completion_tokens int NOT NULL,
    estimated_cost_cents decimal NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_usage_user_created ON ai_usage(user_id, created_at);

-- exercise_aliases: user_id NULL = global alias, user_id set = user alias
CREATE TABLE exercise_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    alias_key text NOT NULL,
    variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_exercise_aliases_global ON exercise_aliases (alias_key) WHERE user_id IS NULL;
CREATE UNIQUE INDEX idx_exercise_aliases_user ON exercise_aliases (user_id, alias_key) WHERE user_id IS NOT NULL;
CREATE INDEX idx_exercise_aliases_lookup ON exercise_aliases (alias_key);

CREATE TABLE chat_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_chat_messages_user_created ON chat_messages(user_id, created_at DESC);

-- Triggers (set_updated_at)
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
