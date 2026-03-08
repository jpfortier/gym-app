-- Users
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id text UNIQUE NOT NULL,
    email text,
    name text,
    photo_url text,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Exercise categories (global: user_id null, user: user_id set)
CREATE TABLE exercise_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);
CREATE UNIQUE INDEX idx_exercise_categories_global_name ON exercise_categories (name) WHERE user_id IS NULL;

-- Exercise variants (one per category)
CREATE TABLE exercise_variants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id uuid NOT NULL REFERENCES exercise_categories(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (category_id, user_id, name)
);

-- Workout sessions (one per user per day)
CREATE TABLE workout_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, date)
);

-- Log entries (block - one per exercise in session)
CREATE TABLE log_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE RESTRICT,
    notes text,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_log_entries_session ON log_entries(session_id);
CREATE INDEX idx_log_entries_exercise_created ON log_entries(exercise_variant_id, created_at);

-- Log entry sets (one per set within block)
CREATE TABLE log_entry_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    log_entry_id uuid NOT NULL REFERENCES log_entries(id) ON DELETE CASCADE,
    weight decimal NOT NULL,
    reps int NOT NULL,
    set_order int NOT NULL,
    set_type text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_log_entry_sets_log_entry ON log_entry_sets(log_entry_id);

-- Notes (global or scoped)
CREATE TABLE notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_category_id uuid REFERENCES exercise_categories(id) ON DELETE CASCADE,
    exercise_variant_id uuid REFERENCES exercise_variants(id) ON DELETE CASCADE,
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Personal records
CREATE TABLE personal_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE RESTRICT,
    pr_type text NOT NULL,
    weight decimal NOT NULL,
    reps int,
    log_entry_set_id uuid REFERENCES log_entry_sets(id) ON DELETE SET NULL,
    image_url text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_personal_records_user_exercise ON personal_records(user_id, exercise_variant_id, pr_type);

-- AI usage tracking
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

-- Workout sessions index
CREATE INDEX idx_workout_sessions_user_date ON workout_sessions(user_id, date);
