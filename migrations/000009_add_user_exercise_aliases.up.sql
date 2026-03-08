-- User exercise aliases: learned mappings from user input (e.g. "rdl") to resolved variant.
-- When we resolve via embedding or create, we store the alias so future lookups skip the LLM.
CREATE TABLE user_exercise_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alias_key text NOT NULL,
    variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, alias_key)
);

CREATE INDEX idx_user_exercise_aliases_user_alias ON user_exercise_aliases (user_id, alias_key);
