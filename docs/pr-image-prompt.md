# PR Image Prompt (gpt-image-1.5 Edit API)

Reference: `samples/trains/` — character sheet + context examples.

Uses Images Edit API (`POST /v1/images/edits`) with reference images uploaded via Files API.
File IDs in `GYM_PR_IMAGE_REF_1`, `GYM_PR_IMAGE_REF_2`.

## Style (from samples)

- **Character:** Anthropomorphic train locomotive — yellow/orange diesel engine with muscular human arms, determined face, smokestack
- **Action:** Bench pressing (or performing the actual lift) — barbell or train freight cars as weights
- **Weights:** Train freight cars with the PR weight in large numbers (e.g. "140" on red cars)
- **Setting:** Industrial gym / warehouse / train yard — gritty, concrete, metal, strip lights
- **Aesthetic:** Bold cartoon/comic style, slight vintage or grunge texture overlay
- **Color:** Yellow/orange train, red/blue freight cars, industrial grays
- **PR cues:** Numbers on weights, optional "NEW PR!" speech bubble, optional blackboard with previous/next weights

## Dynamic variables (inject into prompt)

- `exercise_name` — e.g. "Bench Press", "Romanian Deadlift"
- `weight` — e.g. 140, 185
- `reps` — e.g. 1 (for 1RM), 6 (for natural set)
- `pr_type` — "one_rep_max" | "natural_set"
- `date` — optional, for display

## Example prompt template

```
Cartoon illustration, anthropomorphic yellow and orange train locomotive with muscular arms bench pressing a barbell. The weights are red train freight cars with "{{weight}}" in large yellow numbers. Industrial warehouse gym setting, concrete floor, gritty texture. Bold comic book style. Celebrating a new personal record.
```

For other exercises, swap "bench pressing" with the lift (e.g. "deadlifting", "squatting", "overhead pressing").

## Notes

- Keep prompt concise; DALL-E 3 handles detail well
- Exercise-specific: adapt the lift pose to match (deadlift = pulling from floor, squat = standing with bar, etc.)
- Numbers must be legible — "{{weight}}" in prompt so DALL-E renders it
- **Character + variation:** Same train character across PRs, but each image is a fresh generation. Variation (weird, different, silly) is good.
- **Numbers:** Trust DALL-E; it usually gets them right. No post-processing overlay for v1.
- **Bodyweight:** Use reps as the primary number (e.g. "15" for pull-ups). Weight omitted or "bodyweight" in prompt.
