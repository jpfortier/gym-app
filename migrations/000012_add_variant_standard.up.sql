-- Add standard flag: only one variant per category has standard=true (the default when user doesn't specify).
ALTER TABLE exercise_variants ADD COLUMN standard boolean NOT NULL DEFAULT false;

-- Mark existing "standard" named variants as the default.
UPDATE exercise_variants SET standard = true WHERE name = 'standard' AND user_id IS NULL;

-- Ensure at most one standard per (category_id, user_id).
CREATE UNIQUE INDEX idx_exercise_variants_one_standard_per_category
  ON exercise_variants (category_id, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid))
  WHERE standard = true;
