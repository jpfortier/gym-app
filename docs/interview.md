# Gym Log App Interview

## How To Use This

Answer each question as briefly as you want. One line per answer is enough. Short answers are better than perfect answers.

I will ask these one at a time in chat. Each answer will be copied back into this file under the matching question.

**Context:** See `input.md` and `chatgpt-workout-log-chat-history.md` for current ChatGPT workflow. In that app I always have to say "current day"—the new app should infer that; I don't want to specify it.

Example answer styles:

- "Yes"
- "No"
- "Probably"
- "Must have"
- "Nice to have"
- "Android only for now"

## Product Goal

1. What is the single most important job of the app on day one?
   Answer: Keep track of workouts and help during a workout to know how much to lift. Most usage will happen during the workout period, with logging and querying usually happening within an hour of each other.
2. What would make you stop using the ChatGPT-based workflow and switch to this app full time?
   Answer: I would switch immediately if I had it because I want to use my own tool instead of the ChatGPT-based workflow.
3. Is this primarily for your own use first, or should version one already feel ready for public users?
   Answer: It is mainly for me and my friends. This is not intended to be a major public project right now.

## Platform

4. Should version one be Android only, or should we plan for iPhone too?
   Answer: Version 1 should be Android only. Make it an Android app.
5. Do you want a mobile app only, or also a web app/admin panel early on?
   Answer: Yes, we need a web admin panel for backend administration: view data, create/delete users, administrative tasks. The Android app handles workout logging; admin panel is not for logging.
6. Is offline logging important for version one?
   Answer: No. I'll have WiFi almost all the time.

## Logging Flow

7. When you speak a log like "bench press 140", what details do you usually mean by default?
   Answer: Today's date; I'm usually logging in the moment unless I specify otherwise. "Bench press" is a high-level category and defaults to standard bench press. If I say "close grip bench press", "floor bench", or "dumbbell bench press", those are variants and usually different weights.
8. Do you want each spoken entry to create one set, a full exercise entry, or should the app ask follow-up questions when unclear?
   Answer: Group entries into the same day/session. If I'm logging on March 6th in the morning, assume it's the same session for that day. Don't ask—just assume I'm at the gym that day. If I need something different, I'll specify (e.g., "add this to yesterday's log").
9. Do you care about reps from the start, or is weight-only logging acceptable at first?
   Answer: Typically weight and reps. Examples: "bench press 140, four sets of eight" or "sets of four at 140, then max one at 160".
10. Do you want to track warm-up sets, working sets, and drop sets separately?
   Answer: Up to the person logging. They describe what they did; the system stores it.
11. Should the app assume you are inside an active workout session, or should logging work even without explicitly starting a session?
   Answer: When I first log something, assume it's today and create a new workout session automatically. No need to confirm.
12. After each voice log, should the app confirm what it understood before saving?
   Answer: No. Just show the result of what was created. I can always click a button to edit or re-record.

## Query Flow

13. When you ask "what's my last shoulder press?", what answer format do you want?
   Answer: Show the date and the last entry, possibly the last two. Natural summary, e.g., "100 pound dumbbell" and "the week before you did barbell 120 pound for eight reps in two sets" or whatever was logged.
14. How many past results do you usually want to see: 1, 3, 5, or all recent?
   Answer: Depends on the question. Main interface is chat-like. For "what's my most recent deadlift": show a tile with deadlift, imagery (to decide later), and the number. Swipe left/right or icon to go back in history for that exercise. Can expand from RDL up to all deadlifts.
15. Should the answer prioritize the exact variation first, then show related lifts under the parent category?
   Answer: Exact variation is most important. If I ask for RDLs, show RDLs. Could also show related lifts. Most recent is what I need—if I do RDLs every week, the most recent one matters most.
16. Do you want a dedicated query mode and log mode, or should the app infer your intent automatically?
   Answer: Infer intent automatically.
17. Is voice response important, or is on-screen text enough?
   Answer: On-screen only.

## Exercise Structure

18. Should exercises start from a predefined library, or be mostly created dynamically from your speech?
   Answer: Have a list of known exercises, but keep it dynamic. I can't know what people will do—they might do 100 pound hula hoops over their heads. Don't limit them.
19. Who should be allowed to create a new exercise variation: the AI automatically, the user manually, or both?
   Answer: AI does it when the person says they did something. Just do it.
20. If the AI is unsure whether a variation already exists, should it guess, ask, or save under a temporary uncategorized bucket?
   Answer: Save what they said. Provide a mechanism for the user to correct if needed.
21. Should one variation be allowed to belong to more than one top-level category, or always exactly one?
   Answer: One-to-one is fine. Close grip bench press and close grip barbell shoulder press are different exercises with different parents.

## Data Model

22. For each set, which fields matter on day one: weight, reps, sets, notes, timestamp, effort/RPE, rest time, side, tempo?
   Answer: Support everything and anything. Have categories for expected fields; if I need more later, I can add them in code. Come up with a good core set and stick with it.
23. Do you want bodyweight exercises tracked too?
   Answer: Any exercises. Don't limit.
24. Do you want cardio or non-lifting workouts included in the same system?
   Answer: No.
25. Should workout notes or freeform comments be supported from the start?
   Answer: Yes. Two levels: global (things to remember) and scoped. Notes can apply to a top-level category (e.g., all RDLs) or to a specific variant. Example: logging a specific RDL variant but the note applies to all RDLs.

## Reports And Insights

26. What are the top 3 reports or views you know you would actually use?
   Answer: (1) Timeline—scroll back and see what I've done. (2) PR section—see PRs over time, maybe part of timeline. (3) Category section—zoom out to see exercise types (bench press, deadlifts, shoulder press, pull-ups, push-ups), drill down into variations. (4) Possibly another user view to see other people on the platform.
27. Should the home screen show today's workout, recent activity, or progress summaries?
   Answer: Go right into chat mode. Pop the app and start asking or logging. Don't expect much browsing—get to business. Can navigate to other screens from there.
28. Do you want charts early, or is a clean list/table view enough for version one?
   Answer: Charts are v2. List/table view for v1.
29. Is tracking personal records enough at first, or do you also want streaks, volume trends, and muscle-group summaries?
   Answer: Just logging activity. Not interested in streaks, volume trends, or muscle-group summaries.

## Personal Records

30. How do you define a PR: highest weight ever, highest estimated 1RM, most reps at a weight, or several PR types?
   Answer: Several PR types. Main two: highest weight for a natural set, and one rep max. Could add more later. Need a sensible default—don't want to always ask "which one was that?"
31. Should PR detection happen instantly when logging, or later in the background?
   Answer: When logging—if we detect a PR or the person mentions it, create it then.
32. Should every PR generate an image, or only major PRs?
   Answer: Every PR.
33. Do you want PR images to be private by default, shareable, or automatically postable later?
   Answer: Private by default, in your account. Sharing would be nice (WhatsApp, etc.). V2: built-in chat between users or post to a channel for others on the app.

## Users And Accounts

34. Do you want email/password login, social login, magic link, or something else?
   Answer: Magic link and Google social login. *(Later: Google only, no magic link.)*
35. Will most users be individuals only, or do you eventually want coaches/clients or shared programs?
   Answer: Just individuals.
36. Do users need profile settings like units, gym name, goals, or preferred exercise names?
   Answer: Minimal—just name, maybe photo. Same gym, pounds only. No goals, no preferred exercise names.

## AI Behavior

37. Where do you want AI to help most: parsing logs, answering queries, suggesting categories, generating summaries, or all of those?
   Answer: Taking in what the person says—either a question or reporting what they did. Secondary: correcting recent entries (e.g., "that last one was 165, not 135"). Mainly logging and answering.
38. What mistakes would be most annoying: wrong exercise mapping, wrong weight, wrong PR detection, or noisy follow-up questions?
   Answer: Avoid follow-up questions—we're logging after heavy weight, don't get chatty. Wrong PR detection is annoying (big flashy PR when it wasn't). Missing or losing records would be really annoying—don't do that. Mishearing is out of our control (speech-to-text).
39. Should the app be conservative and ask more clarifying questions, or move fast and let you edit mistakes later?
   Answer: Move fast. Edit mistakes later.

## Visual Direction

40. Which matters more for version one: polished animations or very fast/simple interaction?
   Answer: Fast and simple. PR celebration should animate.
41. Do you want the visual style to feel serious, playful, intense, futuristic, or something else?
   Answer: Dark mode. Airbnb-style cards (collectible, illustrated, rounded corners) when completing a reservation. Trains theme for PRs—anthropomorphic train characters. PR cards flip over to reveal the card you've won. Reduce form filling as much as possible.
42. Are there any apps whose UI feel you want to borrow from?
   Answer: No. Don't use too many apps.

## Scope Control

43. What features are definitely not needed in version one?
   Answer: Charting. Full user profile or fluff. Keep UI bare bones. Need: basic admin tool, AI token usage tracking, view data globally and as a particular user.
44. What features would be embarrassing to launch without?
   Answer: Persist logs, don't lose data. AI must not accidentally delete everything (e.g., "get rid of that last one" shouldn't wipe the whole entry). Basic undo. Soft delete—removing just disables, doesn't hard delete. Routine to clear disabled entries at end of week or similar.
45. If we had to build a usable MVP in the smallest possible scope, what would the bare minimum include?
   Answer: What I do with ChatGPT now: say "this is my workout" to create an entry, and ask questions to find my last workout for an exercise.

## Final Priorities

46. Rank these from most important to least important for version one: voice logging, querying history, reports, PR celebrations, multi-user accounts.
   Answer: That order is correct. (Note: I use voice-to-text on Mac—support keyboard/voice input, not necessarily phone mic only.)
47. What is the biggest technical or product risk you are worried about right now?
   Answer: None major. Maybe distribution—Google Play Store vs passing around APK. (Note: 20+ years web dev, haven't built many apps.)
48. What would success look like after the first month of using the app?
   Answer: At least the basics working. Hopefully more advanced stuff too.
