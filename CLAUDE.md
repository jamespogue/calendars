# Calendars

Subscribable iCalendar (.ics) files for Lewis University courses and related schedules, published with GitHub Pages at https://jamespogue.github.io/calendars/. People subscribe to these URLs, so treat every pushed file as a public, live link.

## Published files

- **Never rename or delete a pushed .ics file**, even an obvious mistake (e.g. `f26-cpsc-440000-001.ics`), because subscribers would lose it.
- A file may be deleted only when **every date in it has passed** (judge by its last event, not the term's end) **and** the user explicitly approves deleting that specific file.
- Unpushed files can be renamed freely.

## Naming

- New files: `YYYY-<term>-<name>.ics`, where term is `sp`, `su`, or `fa`. Examples: `2027-sp-academic-calendar.ics`, `2027-sp-cpsc-44000-001.ics`.
- Older `f26-*` files keep their names (see above).

## File format

- RFC 5545 with **CRLF line endings**. Preserve them when editing (e.g. Python with `newline=""`); don't use `sed -i` on .ics files. `.gitattributes` marks `*.ics` as `-text` so git stores them byte-for-byte (GitHub Pages serves the stored bytes); everything else is LF.
- Header: `PRODID:-//Lewis University//<description>//EN`, `X-WR-CALNAME:<TERM> <name>` (e.g. `SP27 ...`).
- Every event has a unique `UID` and a `DTSTAMP`.
- **Class meetings:** `TZID=America/Chicago` times, weekly `RRULE` with `UNTIL`, `EXDATE` for every no-class day, and a 15-minute `VALARM` reminder. Final exams are separate events with the same reminder.
- **All-day events** (holidays, breaks, semester dates): `VALUE=DATE`, exclusive `DTEND` (day after the last day), `TRANSP:TRANSPARENT`, `STATUS:CONFIRMED`, no alarm.

## Dates

- Source: Lewis academic calendar, https://www.lewisu.edu/academics/academiccalendar.htm. The page lists several years; read the correct term's section.
- Lewis doesn't publish finals week on that page. Confirm exam dates with the user.

## Editor settings

- `.editorconfig` sets tabs, LF, and a final newline everywhere, with CRLF for `.ics`. Keep it and `.gitattributes` in agreement.

## Formatting

- Prettier formats HTML, Markdown, and JSON (`.prettierrc`: tabs, 120-column `printWidth`, and `prettier-plugin-tailwindcss`). `.ics` files are excluded in `.prettierignore`.
- Run `npm install` once after cloning. Then `npm run format` after editing, and `npm run check` before committing; it must pass.
- Versions are pinned exactly in `package.json`. `node_modules/` is gitignored.

## Keeping things in sync

When adding or removing a calendar, update both:

- `README.md` (list of calendar URLs)
- `index.html` (the `GROUPS` list in the script; the page uses Tailwind via the pinned CDN browser build)
