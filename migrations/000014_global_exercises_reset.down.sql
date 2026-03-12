-- Restore user_exercise_aliases and remove exercise_aliases
DROP TABLE IF EXISTS exercise_aliases;

CREATE TABLE user_exercise_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alias_key text NOT NULL,
    variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, alias_key)
);
CREATE INDEX idx_user_exercise_aliases_user_alias ON user_exercise_aliases (user_id, alias_key);
