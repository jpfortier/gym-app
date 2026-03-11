DROP INDEX IF EXISTS idx_exercise_variants_one_standard_per_category ON exercise_variants;
ALTER TABLE exercise_variants DROP COLUMN IF EXISTS standard;
