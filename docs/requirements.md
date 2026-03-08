# Gym Log App Requirements

## Purpose

Build an Android app for tracking gym workout logs through voice-first input and fast retrieval of recent workout history. Replace the current workflow of using a persistent ChatGPT conversation for logging workouts and asking about recent lifts.

**Primary job:** Keep track of workouts and help during a workout to know how much to lift. Most usage happens during the workout period, with logging and querying usually within an hour of each other.

**Audience:** Me and my friends. Not a major public project.

**Context:** See `input.md` and `chatgpt-workout-log-chat-history.md` for current ChatGPT workflow. In that app I always have to say "current day"—the new app must infer that.

---

## Platform

- **Android only** for version 1
- **Web admin panel** for backend administration: view data, create/delete users, AI token usage tracking, view globally and as a particular user. Admin panel is not for logging.
- **No offline** for v1—WiFi assumed

---

## Core Product Requirements

### 1. Voice-First Logging

- Support audio and keyboard/voice-to-text input (user uses voice-to-text on Mac; not phone mic only)
- Convert spoken workout entries into text
- Interpret natural speech about exercises and weights
- Log workout activity with minimal friction during a workout session
- **No confirmation before saving**—show the result of what was created; user can edit or re-record
- **Move fast**—edit mistakes later; avoid follow-up questions (logging after heavy weight, don't get chatty)

### 2. Defaults and Session Behavior

- **Date:** Assume today; infer current day—user should not have to specify
- **Session:** When first log of the day, assume today and create a new workout session automatically. No need to confirm.
- **Grouping:** Group entries into the same day/session. If user needs something different, they'll specify (e.g., "add this to yesterday's log")
- **Exercise defaults:** "Bench press" = high-level category, defaults to standard bench press. "Close grip bench press", "floor bench", "dumbbell bench press" = variants with different weights

### 3. Log Entry Content

- **Weight and reps** from the start. Examples: "bench press 140, four sets of eight" or "sets of four at 140, then max one at 160"
- **Set types:** Up to the person—warm-up, working, drop sets—they describe it, system stores it
- **Extensible fields:** Support everything. Have categories for expected fields (weight, reps, sets, notes, timestamp, effort/RPE, rest time, side, tempo); user can add more in code later
- **Bodyweight:** Yes. Any exercises. Don't limit.
- **Cardio:** No. Lifting only.

### 4. Notes and Comments

- **Two levels:** Global (things to remember) and scoped
- Notes can apply to a top-level category (e.g., all RDLs) or a specific variant
- Example: logging a specific RDL variant but the note applies to all RDLs

### 5. Query Mode

- **Infer intent automatically**—no dedicated query vs log mode; single chat interface
- **Answer format:** Show date and last entry, possibly last two. Natural summary (e.g., "100 pound dumbbell" and "the week before you did barbell 120 pound for eight reps in two sets")
- **Interface:** Chat-like. For "what's my most recent deadlift": show a tile with exercise, imagery, and the number. Swipe left/right or icon to go back in history for that exercise. Can expand from RDL up to all deadlifts
- **Exact variation first**—if user asks for RDLs, show RDLs. Most recent matters most. Can also show related lifts
- **On-screen only**—no voice response

### 6. Exercise Hierarchy and Variations

- **Structure:** One variation belongs to exactly one top-level category (one-to-one). Close grip bench press ≠ close grip barbell shoulder press—different parents
- **Library + dynamic:** Have a list of known exercises, but keep it dynamic. Don't limit—people might do "100 pound hula hoops over their heads"
- **AI creates variations** when the person says they did something. Just do it.
- **When unsure:** Save what they said. Provide a mechanism for the user to correct if needed

### 7. Search and Retrieval

- Support top-level category (e.g., Deadlift → all variations) and specific variation (e.g., RDL only)
- Semantic retrieval or vector store to understand relationships between parent and child exercises
- User should not have to remember exact stored variation names

### 8. Data Integrity (Critical)

- **Persist logs.** Don't lose data.
- **AI must not accidentally delete everything**—e.g., "get rid of that last one" must not wipe the whole entry
- **Basic undo.** Soft delete—removing just disables, doesn't hard delete
- **Cleanup routine** to clear disabled entries at end of week or similar
- **Annoying mistakes to avoid:** Wrong PR detection (big flashy PR when it wasn't), missing/losing records. Mishearing is out of our control (speech-to-text)

### 9. Multi-User Support

- Multiple users, each with own account and data
- **Auth:** Google Sign-In only (no Clerk, no magic link)
- **Profile:** Minimal—name, maybe photo. Same gym, pounds only. No goals, no preferred exercise names
- **Scope:** Individuals only. No coaches/clients or shared programs for v1

### 10. Reports and History Views

- **Timeline**—scroll back and see what I've done
- **PR section**—see PRs over time, maybe part of timeline
- **Category section**—zoom out to see exercise types (bench press, deadlifts, shoulder press, pull-ups, push-ups), drill down into variations
- **Other user view** (admin)—see other people on the platform
- **Charts:** v2. List/table view for v1
- **No streaks, volume trends, or muscle-group summaries**

### 11. Home / Entry Point

- **Go right into chat mode.** Pop the app and start asking or logging. Don't expect much browsing—get to business. Navigate to other screens from there.

### 12. Personal Records (PRs)

- **PR types:** Several. Main two: highest weight for a natural set, and one rep max. Sensible default—don't ask "which one was that?"
- **Detection:** When logging—if we detect a PR or the person mentions it, create it then
- **Every PR generates an image**
- **Images:** Private by default, in account. Sharing nice (WhatsApp). V2: built-in chat or post to channel for others
- **Visual:** Trains theme—anthropomorphic train characters. Airbnb-style cards (collectible, illustrated, rounded corners). PR cards flip over to reveal the card you've won. PR celebration should animate.

### 13. AI Behavior

- **Primary:** Taking in what the person says—either a question or reporting what they did
- **Secondary:** Correcting recent entries (e.g., "that last one was 165, not 135")
- **No:** Suggesting categories, generating summaries—mainly logging and answering

---

## UX and Design Direction

- **Dark mode**
- **Fast and simple** for v1. PR celebration animates
- **Airbnb-style cards** for PRs—collectible, illustrated, rounded corners
- **Trains theme** for PRs—anthropomorphic train characters; flip-over reveal
- **Reduce form filling** as much as possible
- **Bare bones UI**—no full user profile or fluff

---

## Admin Panel Requirements

- View data globally and as a particular user
- Create/delete users
- AI token usage tracking
- Administrative tasks

---

## Feature Priorities (v1, most to least important)

1. Voice logging
2. Querying history
3. Reports
4. PR celebrations
5. Multi-user accounts

---

## MVP Bare Minimum

What I do with ChatGPT now: say "this is my workout" to create an entry, and ask questions to find my last workout for an exercise.

---

## Out of Scope for v1

- Charting
- Full user profile / fluff
- Offline logging
- iPhone
- Cardio
- Streaks, volume trends, muscle-group summaries
- Coaches/clients, shared programs

---

## Tech Stack (Decided)

| Layer | Choice |
|-------|--------|
| **Backend** | Go |
| **Database** | Postgres (Fly Postgres) |
| **Hosting** | Fly.io |
| **Auth** | Google Sign-In (direct, no Clerk) |
| **Distribution** | APK via S3 or R2. In-app update flow. Play Store later (ID verification pending). |
| **Media storage** | Cloudflare R2. PR images, user photos. Presigned URLs for access. FCM for notifications. |
| **Admin panel** | Alpine.js + Go templates. Same backend. Dashboard + table CRUD. Roles: user, coach, owner, admin. |

---

## Technical Notes

- **Background:** 20+ years web dev; haven't built many apps

---

## Success Criteria (First Month)

At least the basics working. Hopefully more advanced stuff too.

---

## Major Product Areas

- Authentication (Google Sign-In)
- Audio capture / speech-to-text
- Natural-language interpretation
- Exercise standardization and hierarchy management
- Workout logging
- Query and retrieval
- Reports and history (timeline, PR section, category section)
- PR detection and AI image generation
- Media storage
- Backend and cloud data storage
- Admin panel

---

