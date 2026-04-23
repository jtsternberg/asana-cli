# Asana CLI v4 — Safety & Workflow Design

**Status:** Approved design, ready for implementation planning
**Target release:** v4.0.0 (breaking)
**Author of design:** JT + Claude
**Date:** 2026-04-23

---

## Context

A recent real-world workflow (quarterly rocks planning) exercised the CLI under conditions it wasn't built for: many tasks created in one pass, identity disambiguation across similarly-named users, staged unassigned-then-assign flow to control notification timing, and post-action verification across many task IDs. The CLI made this possible but fragile. Several silent failure modes surfaced, most notably:

- Partial-match user resolution silently picked the wrong user when multiple users shared a first name.
- Partial-match project/section resolution could silently land tasks in the wrong project.
- No first-class way to create tasks unassigned, assign later, and verify the resulting state.
- No structured error reporting — ambiguity and not-found conditions emerged as ad-hoc error strings.
- No idempotent create, so reruns after partial failure duplicated tasks.

This document specifies the minimum set of changes to make the CLI safe and predictable for these workflows without inflating command surface or adding use-case-specific features.

**Non-goals explicitly ruled out during design:**
- No `tasks bulk ...` subcommand tree. Agent/shell loops with improved single-task commands handle multi-item operations; partial-apply risk is handled by reading per-call output and retrying.
- No markdown-to-task converter. Too workflow-specific for a general CLI.
- No YAML spec format. No spec files at all.
- No changes to auth, time tracking, workspaces, teams, tags, or the interactive prompter.

---

## Section 1 — Overview & goals

1. **Identity resolution becomes strict** across users, projects, and sections. Case-insensitive exact match wins; ambiguous or unknown names fail loudly with a candidate list and remediation hints. Explicit ID/email flags (`--assignee-email`, `--project-id`, etc.) exist for scripts that want type safety. No partial/substring matching. No silent first-match picking.
2. **`tasks create` gains `--force-unassigned`** (Stage-A guardrail — blocks `--assignee*` at parse time) and **`--skip-if-exists`** (idempotent reruns via title-in-scope match).
3. **`tasks update` gains `--add-followers`, `--remove-followers`, and `--unassign`** to cover follower management and reassignment-to-unassigned.
4. **New read command `tasks audit`** produces a structured JSON report of assignee + follower + section + due-date state for a project/section scope or a list of task IDs. Designed for post-action verification by agents and humans.
5. **Errors use a normalized code enum** (`USER_AMBIGUOUS`, `PROJECT_NOT_FOUND`, `CONFLICTING_FLAGS`, etc.) with a structured JSON envelope containing `code`, `kind`, `input`, `candidates[]`, and `remediation[]`. Exit codes split `usage` (2), `resolution` (3), and `permission` (4) from generic failure (1).
6. **The Claude plugin** (`claude-plugin/`) ships updated skill references, a new Stage-A/Stage-B workflow reference, a terminology-translation rule (user says "collaborator" → CLI means `--followers`), and refreshed agent guidelines that include structured-error parsing and audit-based verification.

**Release:** v4.0.0. Breaking because strict resolution removes silent partial-match behavior that existing scripts may rely on. Loud failure is the win. Migration recipes documented in `docs/MIGRATION-v4.md`.

---

## Section 2 — Architecture: centralized resolver

A new package `internal/resolve/` owns every name/email/ID → object lookup. Every command calls into it; no command does its own partial-match loop. This is the single load-bearing architectural decision — if the resolver is correct and well-tested, every command that calls it gets correctness for free.

### Public API (sketch)

```go
package resolve

type Resolver struct {
    client      *asana.Client
    workspaceID string
    // lazy, per-resolver caches; one fetch per type per resolver lifetime
    users    []*asana.User
    projects []*asana.Project
    sections map[string][]*asana.Section // keyed by project ID
    me       *asana.User
}

func New(client *asana.Client, workspaceID string) *Resolver

// Primary API. Callers pass a Ref carrying the raw value + optional type hint.
func (r *Resolver) User(ref UserRef) (*asana.User, error)
func (r *Resolver) Users(refs []UserRef) ([]*asana.User, error) // atomic: all-or-none
func (r *Resolver) Project(ref Ref) (*asana.Project, error)
func (r *Resolver) Section(projectID string, ref Ref) (*asana.Section, error)

type Ref struct {
    Value string
    Hint  TypeHint // Auto | Name | ID
}

type UserRef struct {
    Value string
    Hint  TypeHint // Auto | Name | ID | Email
}

type TypeHint int
const (
    TypeAuto TypeHint = iota
    TypeName
    TypeID
    TypeEmail // user-only
)
```

### Auto-detect rules (when hint is `Auto`)

- Value contains `@` → Email (user); for project/section callers, `@` in name is invalid → `INVALID_*` error (names can't contain `@` in practice).
- Value is all digits → ID.
- Otherwise → Name.

Explicit flags (`--assignee-email`, `--assignee-id`, `--project-id`, `--section-id`, `--followers-email`, `--followers-id`) set the hint explicitly and pre-validate the format. `--assignee-email alex` errors `INVALID_EMAIL` before any API call.

### Resolution rule (unified across all types)

1. **ID path:** direct match against the fetched list. Miss → `{KIND}_NOT_FOUND`.
2. **Email path (user only):** exact case-insensitive match on `.email`. Miss → `USER_NOT_FOUND`. Multi-match (defensive; shouldn't occur at Asana level) → `USER_AMBIGUOUS`.
3. **Name path:** case-insensitive exact match on `.name`.
   - Zero → `{KIND}_NOT_FOUND` with up-to-3 "did you mean" suggestions (Levenshtein distance ≤ 2).
   - Exactly one → return it.
   - Multiple (case-variant collisions like "Rocks" + "rocks") → `{KIND}_AMBIGUOUS` with full candidate list.
4. **No partial/substring matching at any layer.** This is the behavior change.

### `me` shorthand

`User(UserRef{Value: "me"})` resolves via `client.CurrentUser()` on first call, caches the result for the resolver's lifetime. `me` is preserved as user-only shorthand and bypasses normal name/email/ID routing.

### Error type

```go
type ResolveError struct {
    Code       string       // "USER_AMBIGUOUS", "PROJECT_NOT_FOUND", etc.
    Kind       string       // "user" | "project" | "section" | "task" | "" (for flag errors)
    Input      string       // raw string the user passed
    Candidates []Candidate  // populated for *_AMBIGUOUS; holds "did you mean" for *_NOT_FOUND
    Hints      []string     // human-readable remediation lines
}

type Candidate struct {
    ID    string
    Name  string
    Email string // users only; omitted for projects/sections
}
```

Implements `error`. Commands pass it up unchanged; the root error handler formats it for text or `--json` stderr output (Section 5).

### Caching + pagination

The resolver lazy-fetches each resource list on first use and caches for its lifetime (one command invocation). Uses the existing `AllUsers` / `AllProjects` helpers in `internal/api/asana/`. Adds a new `AllSections(client, ...)` helper on `Project` that mirrors the pattern (current `Sections()` is paginated but unwrapped).

Benefit beyond safety: a single `tasks create -a X -p Y -s Z -f "a,b,c"` invocation that today re-fetches users once for assignee resolution and again for followers resolution now fetches users once.

### Wiring plan

| Command | Before | After |
|---|---|---|
| `tasks create` | 4 inline partial-match loops | `resolver.User`, `.Project`, `.Section`, `.Users(followers)` |
| `tasks update` | 2 inline loops | same |
| `tasks move` | 2 inline loops | `resolver.Project`, `.Section` |
| `tasks search --assignee` | 1 inline loop | `resolver.User` |
| `projects tasks <project>` | 1 inline loop | `resolver.Project` |
| `projects sections <project>` | 1 inline loop | `resolver.Project` |
| `tasks audit` (new) | n/a | `resolver.Project`, `resolver.Section` |

Factory gains a new provider: `f.Resolver() (*resolve.Resolver, error)`. Construction is cheap. Shared across helpers within one command run.

---

## Section 3 — Per-command flag changes

Every write command that resolves names uses the same explicit-mirror pattern. The short flag auto-detects (95% path); explicit mirrors exist for scripts that want type safety. All resolution paths route through `internal/resolve/` so behavior is identical across commands.

### `tasks create`

**New flags:**

| Flag | Purpose |
|---|---|
| `--force-unassigned` | Creates task with no assignee. Parse-time error (`CONFLICTING_FLAGS`, exit 2) if combined with `--assignee` / `--assignee-email` / `--assignee-id`. |
| `--skip-if-exists` | Idempotency. Scope follows `-p` / `-s` (section narrows scope; no `-p` → parse-time `SCOPE_REQUIRED`). Exits 0 on skip; emits `{"status":"already_exists","task_id":"..."}` in JSON mode. |
| `--match-case` | Modifies `--skip-if-exists` to require exact-case title match. Default is case-insensitive. |
| `--include-completed` | Modifies `--skip-if-exists` to include completed tasks when checking existence. Default: uncompleted only. |
| `--assignee-email <email>` | Explicit email mirror for `-a`. Pre-validates `@` present. |
| `--assignee-id <id>` | Explicit ID mirror for `-a`. Pre-validates all-digits. |
| `--project-id <id>` | Explicit ID mirror for `-p`. |
| `--section-id <id>` | Explicit ID mirror for `-s`. |
| `--followers-email <csv>` | Explicit email mirror list. |
| `--followers-id <csv>` | Explicit ID mirror list. |
| `--json` | Machine-readable result (created task id/url, or skipped task id). |

**Existing flag semantics under strict mode:**
- `-a` / `--assignee`: auto-detect per value — `@` → email, all-digits → ID, else name. Strict resolution. `me` preserved.
- `-p` / `--project`: auto-detect — all-digits → ID, else name. Strict.
- `-s` / `--section`: same as `-p`.
- `-f` / `--followers`: CSV; per-item auto-detect. Atomic (all-or-none; errors aggregate).

**Breaking behavior (from v3):**
- `-a "tia"` (partial) → `USER_NOT_FOUND` with "did you mean" suggestions. Was: silently picked first `strings.Contains` match.
- `-a "Alex"` with two users named "Alex Rivera" and "Alex Romano" → `USER_AMBIGUOUS` with both candidates. Was: silently picked first.
- Same for `-p` and `-s`.

### `tasks update`

**New flags:**

| Flag | Purpose |
|---|---|
| `--add-followers <csv>` | Add specified followers. New primary flag name. |
| `--remove-followers <csv>` | Remove specified followers. Requires new `RemoveFollowers` API method (Section 2 / API additions below). |
| `--unassign` | Remove assignee. Parse-time `CONFLICTING_FLAGS` if combined with `--assignee*`. |
| `--assignee-email`, `--assignee-id`, `--followers-email`, `--followers-id` | Explicit mirrors. |
| `--json` | Machine-readable update result. |

**Existing flag change:** `-f` / `--followers` becomes an alias for `--add-followers` with a deprecation note in `--help`. Old scripts keep working; new docs only show `--add-followers`.

### `tasks move`

**New flags:** `--project-id`, `--section-id` explicit mirrors.
**Existing `-p` / `-s`:** strict resolution via resolver, auto-detect on digits.
**Breaking:** partial project/section name matches now error rather than silently moving to the wrong destination. This is the highest-stakes strict-mode change because the old behavior could silently misdirect a task to a wrong project.

### `tasks search`

Strict resolution applied to `--assignee` and any other name-resolved flag. Implementer spot-check: grep `search.go` for all name-resolution paths (the confirmed site is line 416; apply the same treatment to any others like `--creator`, `--project`, `--tag`).

### `tasks list` / `tasks view` / `tasks delete`

No flag changes expected — these operate on task IDs directly or interactive select. Implementer verifies during implementation; apply resolver treatment if any name-resolved flag exists.

### `projects tasks <project>` / `projects sections <project>`

Positional project argument routes through resolver. Auto-detect on digits; strict otherwise. No new explicit flag — positional args stay minimal. Breaking: partial-match on project name stops working.

### Time tracking (`time create`, `time status`)

Uses interactive `cmdutils.SelectTask`. Implementer spot-check: confirm no non-interactive name-by-flag path exists. If it does, apply the resolver treatment.

### API layer additions

New in `internal/api/asana/tasks.go`:

```go
type RemoveFollowersRequest struct {
    Followers []string `json:"followers"`
}

func (t *Task) RemoveFollowers(client *Client, followerIDs []string) error {
    client.trace("Removing followers from task %q", t.Name)
    return client.post(fmt.Sprintf("/tasks/%s/removeFollowers", t.ID),
        &RemoveFollowersRequest{Followers: followerIDs}, t)
}
```

New helper on `Project` in `internal/api/asana/sections.go`:

```go
// AllSections pages through all sections in a project.
func (p *Project) AllSections(client *Client, options ...*Options) ([]*Section, error) { ... }
```

Pattern mirrors existing `AllUsers` and `AllProjects`.

### Implementer uncertainties flagged

- **Factory provider pattern:** confirm `f.Resolver()` matches existing provider conventions (`f.Client`, `f.Config`).
- **`tasks search` name-resolved flags:** grep caught one site; there may be others.
- **Time tracking non-interactive paths:** not deep-read during design.
- **`projects tasks` positional argument:** assumed it routes through the same resolver as `-p` flags elsewhere; verify.

---

## Section 4 — `tasks audit` (new command)

Read-only report. Dumps assignee + follower + section + due-date state for a scope. Designed for post-action verification by agents and humans.

### Flags

| Flag | Purpose |
|---|---|
| `--project <id\|name>` | Scope to a whole project. Required unless `--task-ids` is used. Strict resolution. |
| `--section <id\|name>` | Narrow scope to a section within `--project`. Requires `--project`. |
| `--task-ids <csv>` | Audit a specific list of task IDs. Mutually exclusive with `--project` / `--section`. |
| `--task-ids-file <path>` | Read IDs from file, one per line. Unions with `--task-ids` (deduped). |
| `--include-completed` | Include completed tasks (default: uncompleted only). |
| `--fields <csv>` | Optional field subset. Default returns the full shape below. |
| `--json` | Structured JSON output. Implied for agents; text output is the default for humans. |

### Output shape (JSON)

```json
{
  "audited_at": "2026-04-23T14:30:00Z",
  "scope": {
    "project": {"id": "...", "name": "Project Alpha"},
    "section": {"id": "...", "name": "Section A"}
  },
  "count": 21,
  "tasks": [
    {
      "id": "1214185449285345",
      "name": "Rock Alpha",
      "permalink_url": "https://app.asana.com/0/.../...",
      "section": {"id": "...", "name": "Section A"},
      "assignee": {
        "id": "...",
        "name": "Alex Rivera",
        "email": "alex@example.com"
      },
      "followers": [
        {"id": "...", "name": "Pat Example", "email": "pat@example.com"}
      ],
      "due_on": "2026-06-30",
      "start_on": null,
      "completed": false,
      "created_at": "2026-04-22T14:20:00Z",
      "modified_at": "2026-04-22T14:25:00Z"
    }
  ]
}
```

- `assignee` is `null` when unassigned.
- `followers` is `[]` when empty (not omitted).
- `email` on assignee/follower omitted only if the API doesn't return it.

### Text output (default, non-JSON)

Compact one-line-per-task table: `id | name | section | assignee | follower count | due_on | completed`. Footer: `N tasks audited`. Long names ellipsized. Good for quick human verification.

### API field mask (perf)

Single Asana API call per scope with:

```
opt_fields=name,permalink_url,completed,due_on,start_on,created_at,modified_at,
  assignee.name,assignee.email,
  followers.name,followers.email,
  memberships.section.name
```

For `--task-ids`, one `/tasks/{id}` request per ID with the same mask, parallelized with **bounded concurrency (5)** to stay inside Asana rate limits. This bounded-concurrency helper is shared with any future parallel-fetch work (ties to existing beads issue `asana-cli-93g`).

For `--section` scope, use the existing `/sections/{id}/tasks` endpoint (`Section.Tasks()` already in the API layer) rather than fetching all project tasks and filtering client-side.

### Exit codes

- `0` — audit completes (even if zero tasks match)
- `1` — genuine error (auth, scope not found, API failure)
- `3` — any partial failure on `--task-ids` (per-task entry carries status)

Empty scope is not an error: returns `{"count": 0, "tasks": []}` and exits 0.

### Partial-failure shape (`--task-ids` with some invalid IDs)

```json
{
  "audited_at": "...",
  "count": 3,
  "tasks": [
    {"id": "1", "status": "ok", ... },
    {"id": "2", "status": "not_found", "error": {"code": "TASK_NOT_FOUND", ...}},
    {"id": "3", "status": "ok", ... }
  ]
}
```

Top-level `count` reflects all requested IDs. Exit code 3 if any non-ok status. Text mode: failed entries show a single red `!` prefix.

---

## Section 5 — Error shape

Cross-cutting. Every strict-mode failure across every command produces the same envelope. Agents parse one shape; humans read one format.

### Normalized error codes

| Code | When |
|---|---|
| `USER_AMBIGUOUS` | Name resolves to 2+ users (case-variant collision or multiple exact matches) |
| `USER_NOT_FOUND` | Name/email/ID resolves to 0 users |
| `PROJECT_AMBIGUOUS` | Name resolves to 2+ projects |
| `PROJECT_NOT_FOUND` | Name/ID resolves to 0 projects |
| `SECTION_AMBIGUOUS` | Name resolves to 2+ sections in the project |
| `SECTION_NOT_FOUND` | Name/ID resolves to 0 sections in the project |
| `TASK_NOT_FOUND` | `--task-ids` includes an ID that 404s |
| `INVALID_EMAIL` | `--assignee-email` / `--followers-email` given a value missing `@` |
| `INVALID_ID` | `--*-id` given a non-digit value |
| `SCOPE_REQUIRED` | `--skip-if-exists` without `--project` |
| `CONFLICTING_FLAGS` | `--force-unassigned` + `--assignee*`, or `--unassign` + `--assignee*` |
| `PERMISSION_DENIED` | Asana API 403 |
| `AUTH_REQUIRED` | No PAT configured or token rejected |
| `API_ERROR` | Any other Asana API failure (rate limit, 5xx, malformed response) |

Codes are stable. Documented in the README and in each write command's `--help`. New codes can be added; existing ones never change semantics.

### JSON error envelope (stderr when `--json` is set)

```json
{
  "error": {
    "code": "USER_AMBIGUOUS",
    "kind": "user",
    "message": "\"Alex\" matched 2 users in this workspace",
    "input": "Alex",
    "candidates": [
      {"id": "111", "name": "Alex Rivera", "email": "alex@example.com"},
      {"id": "222", "name": "Alex Romano", "email": "a.romano@example.com"}
    ],
    "remediation": [
      "Re-run with --assignee-email <email>",
      "Re-run with --assignee-id <id>",
      "Or pass a unique exact name"
    ]
  }
}
```

- `candidates[]` holds disambiguation list for `*_AMBIGUOUS`; up-to-3 Levenshtein suggestions for `*_NOT_FOUND`; empty when no close matches.
- `kind` ∈ {`user`, `project`, `section`, `task`} for scoped errors; `null` for flag/conflict errors.
- `email` on candidates populated only for `kind == "user"`.
- `remediation[]` is an ordered list of actionable fixes. Agents pick the first applicable; humans read top-down.

Rendered as 2-space indented JSON (matches existing `view --json` convention). Written to **stderr**, not stdout.

### Text format (human mode, stderr, colored)

```
error: USER_AMBIGUOUS — "Alex" matched 2 users in this workspace:
  1. Alex Rivera   <alex@example.com>        id=111
  2. Alex Romano   <a.romano@example.com>   id=222

Re-run with one of:
  --assignee-email alex@example.com
  --assignee-id 111
  --assignee "Alex Rivera"       (strict mode requires a unique exact name)
```

`error:` prefix in red. Code in bold. Candidates numbered. Remediation hints are concrete — pre-filled with an actual candidate email/id, not a `<placeholder>` — when the command knows the candidate list.

For `*_NOT_FOUND`:

```
error: USER_NOT_FOUND — "Alx" did not match any user in this workspace.

Did you mean:
  Alex Rivera   <alex@example.com>
  Alex Romano   <a.romano@example.com>

Re-run with --assignee-email, --assignee-id, or the full exact name.
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (including idempotent skip) |
| `1` | Generic failure — network, auth, API, anything not covered below |
| `2` | Usage error — cobra default: bad flags, missing required flag, `CONFLICTING_FLAGS`, `INVALID_EMAIL`, `INVALID_ID`, `SCOPE_REQUIRED` |
| `3` | Resolution error — `*_AMBIGUOUS`, `*_NOT_FOUND`, `TASK_NOT_FOUND` |
| `4` | Permission error — `PERMISSION_DENIED`, `AUTH_REQUIRED` |

Shell scripts can branch on exit code 3 to distinguish "user typo" from "system broken" without parsing JSON.

### Output discipline

- **stdout:** command results only. Never errors.
- **stderr:** all errors (JSON or text), all human-facing status messages ("Created task ...", "Skipped: already exists").
- **Successful write** in JSON mode: structured result on stdout. Errors still go to stderr.

---

## Section 6 — Migration & versioning

This is a breaking change. Loud failure is the win. Migration guide exists primarily as a reference for one user — the repo author — updating their own scripts.

### Version

Ship as **v4.0.0**. Current is v3.1.0. Use existing release tooling (`scripts/` + GoReleaser per recent commit history). `asana upgrade` handles install-method-aware update.

### Breaking-change summary for CHANGELOG

| Change | Scope | Old | New |
|---|---|---|---|
| Strict user resolution | `-a`, `-f`, `--followers`, `--assignee`, `--creator` wherever present | Partial/contains match silently picked first | Exact match; `*_AMBIGUOUS` / `*_NOT_FOUND` |
| Strict project resolution | `-p`, `--project`, positional project args | Partial match silently picked first | Exact match; errors |
| Strict section resolution | `-s`, `--section` | Same | Same |
| Exit code 3 | New | All failures were exit 1 | Resolution errors exit 3 |

### Non-breaking additions

- New flags on `tasks create` (`--force-unassigned`, `--skip-if-exists`, `--match-case`, `--include-completed`, explicit mirrors).
- New flags on `tasks update` (`--add-followers`, `--remove-followers`, `--unassign`, explicit mirrors).
- New command `tasks audit`.
- Structured error envelope in `--json` mode (was: unstructured text).

### Migration recipes (`docs/MIGRATION-v4.md`)

**1. Partial assignee match → email**

```bash
# Before (could hit wrong user):
asana tasks create -n "Foo" -p "Project Alpha" -a "Alex"

# After:
asana tasks create -n "Foo" -p "Project Alpha" --assignee-email alex@example.com
```

**2. Don't know the email? Resolve once upfront**

```bash
ALEX_ID=$(asana users list --json | jq -r '.[] | select(.email=="alex@example.com") | .id')
asana tasks create -n "Foo" -p "Project Alpha" --assignee-id "$ALEX_ID"
```

**3. Ambiguous project name → exact name or ID**

```bash
asana tasks create -n "Foo" -p "Q2 2026 — Project Alpha"    # exact name
asana tasks create -n "Foo" --project-id 1234567890          # explicit ID
```

**4. Structured-error handling in agents/scripts**

```bash
result=$(asana tasks create -n "Foo" -p "Project Alpha" -a "Alex" --json 2>&1)
if [[ $? -eq 3 ]]; then
  code=$(echo "$result" | jq -r '.error.code')
  case "$code" in
    USER_AMBIGUOUS)
      email=$(echo "$result" | jq -r '.error.candidates[0].email')
      asana tasks create -n "Foo" -p "Project Alpha" --assignee-email "$email"
      ;;
    USER_NOT_FOUND) echo "User typo; bailing."; exit 1 ;;
  esac
fi
```

**5. Idempotent batch creates**

```bash
for rock in "Rock Alpha" "Rock Beta" "Rock Gamma"; do
  asana tasks create \
    -n "$rock" \
    -p "Project Alpha" \
    -s "Section A" \
    --force-unassigned \
    --skip-if-exists \
    --json
done
# Reruns are safe; existing tasks are reported as already_exists.
```

### Things that do NOT change

- Flag names (`-a`, `-p`, `-s`, `-f`, etc.)
- Interactive prompts when flags are omitted
- JSON output structure for success cases
- Auth flow, keyring storage, config locations
- Untouched commands: auth, config, time, teams, tags, workspaces

### Deprecation posture

`--followers` on `tasks update` stays functional forever; docs and `--help` promote `--add-followers`. No removal planned. No other deprecations in v4.

### Pre-release verification

1. Full test suite passes.
2. Manual smoke: run a Stage-A → Stage-B → followers → audit flow against a throwaway test project. Record the command history as a canonical recipe for the README.
3. Plugin smoke: drive the full workflow through the `asana-task-manager` agent with the new skill prompts (dev-mapped via the directory-source marketplace — see Section 8).
4. Cut `v4.0.0-rc1`, install via `asana upgrade`, run recipes. Only tag `v4.0.0` after rc passes.

---

## Section 7 — Testing strategy

TDD is the house rule (per `AGENTS.md`). Every new flag, error path, and resolver case gets a failing test first.

### Layering

1. **Resolver unit tests** (`internal/resolve/*_test.go`) — single most important surface. Mock list-fetchers; exercise every rule in the resolution matrix.
2. **Command integration tests** (`pkg/cmd/*/..._test.go`) — existing pattern. Use the `asana_mock.go` client mock. Assert command behavior, not resolver internals.
3. **Golden-file tests** for error envelopes — one file per `(code, output_mode)` pair.
4. **End-to-end smoke recipe** — manual, documented; runs against a throwaway project before tagging.

### Resolver unit test matrix

| Test | Assertion |
|---|---|
| `User({Value: "alex@example.com"})` with one email match | Returns that user |
| `User({Value: "alex@example.com"})` with zero matches | `USER_NOT_FOUND`, `candidates == []` |
| `User({Value: "Alex Rivera"})` with exact name | Returns that user |
| `User({Value: "alex rivera"})` with `Alex Rivera` present | Returns user (case-insensitive) |
| `User({Value: "Alex"})` with `Alex Rivera` + `Alex Romano` | `USER_AMBIGUOUS`, candidates lists both |
| `User({Value: "Alx"})` with `Alex Rivera` + `Alex Romano` | `USER_NOT_FOUND` with top-3 Levenshtein suggestions |
| `User({Value: "123456"})` auto-detected ID, matches | Returns user |
| `User({Value: "123456"})` auto-detected ID, no match | `USER_NOT_FOUND` |
| `User({Value: "Alex", Hint: TypeEmail})` | `INVALID_EMAIL` (pre-lookup) |
| `User({Value: "me"})` | Resolves to `CurrentUser()`, caches |
| `User` called twice on same resolver | Underlying `AllUsers` fetched once |
| `Project({Value: "rocks"})` with `Rocks` + `ROCKS` | `PROJECT_AMBIGUOUS` with both |
| `Section` before `Project` | Either order works (independent caches) |
| `Users([]UserRef{...})` with one failing ref | Aggregate error; no partial resolution returned |

### Command integration tests (new or updated)

- `tasks create --force-unassigned --assignee alex@example.com` → `CONFLICTING_FLAGS`, exit 2, no API call
- `tasks create --skip-if-exists` without `-p` → `SCOPE_REQUIRED`, exit 2
- `tasks create -n "Foo" -p "Project Alpha" -s "Section A" --skip-if-exists` when "Foo" exists in scope → `already_exists`, exit 0, no create call
- `tasks create --skip-if-exists --match-case` with "Foo" and "foo" both present → match-case differentiates; "foo" is created
- `tasks create --skip-if-exists` when existing match is completed → creates (default excludes completed)
- `tasks create --skip-if-exists --include-completed` with completed match → skips
- `tasks update --remove-followers alex@example.com` → calls new `RemoveFollowers` with resolved ID
- `tasks update --unassign --assignee alex@example.com` → `CONFLICTING_FLAGS`
- `tasks update --unassign` → posts assignee=null (or Asana's equivalent shape; implementer verifies)
- `tasks audit --project "Project Alpha" --section "Section A" --json` → envelope matches golden file
- `tasks audit --task-ids 1,2,3` with task 2 returning 404 → hybrid output, exit 3
- `tasks move -p "ambiguous"` → `PROJECT_AMBIGUOUS`, exit 3, no move
- `tasks search --assignee "Alex"` with two Alexes → `USER_AMBIGUOUS`, exit 3

### Golden-file tests for error envelopes

One file per `(code, mode)` pair under `internal/resolve/testdata/errors/`:

```
user_ambiguous.text.golden
user_ambiguous.json.golden
user_not_found.text.golden
user_not_found.json.golden
...
```

Test reads the golden and byte-compares to emitted output for a fixed input. Intentional format changes update the golden.

### Regression tests for behavior we're KEEPING

- `tasks create -a me` still resolves to current user.
- Interactive prompts still fire when flags are omitted.
- `--json` success shapes unchanged (confirm via existing golden tests; add where missing).
- Untouched commands (`asana users list`, etc.) pass existing tests unchanged.

### Perf/rate-limit tests

- `audit --task-ids <50 ids>` issues ≤50 requests with concurrency capped at 5 (verify via mock-client request-timing recorder).
- Resolver caches: within one command, multiple `User()` / `Project()` calls don't re-fetch.

### Not tested

- Real Asana API round-trips. All tests use `asana_mock.go`.
- Interactive prompter flows for new flags (they're flag-only, non-interactive by nature).
- `asana upgrade` v3→v4 path — manual smoke only.

### Coverage target

`internal/resolve/` ~95% line coverage. New command code matching existing pattern. No hard CI gate — implementer runs `go test -cover ./...` and eyeballs each package.

---

## Section 8 — Claude plugin changes

Plugin ships with the repo. v4 publishes skill + agent updates alongside the CLI. Goal: next quarterly-rocks run, the agent has no gaps to fumble over.

### Current layout

```
claude-plugin/
  .claude-plugin/plugin.json   # version needs bump to 4.0.0
  agents/asana-task-manager.md
  skills/using-asana-cli/
    SKILL.md
    references/
      CREATE_TASK.md
      UPDATE_TASK.md
      MOVE_TASK.md
      DELETE_TASK.md
      TROUBLESHOOTING.md
```

### Dev workflow (already in place; documented here for reference)

The plugin is already live-mapped via a directory-source marketplace in `~/.claude/settings.json`:

```json
"asana-cli": {
  "source": { "source": "directory", "path": "/Users/JT/Code/asana-cli" }
}
```

Paired with `.claude-plugin/marketplace.json` at the repo root. Edits to `claude-plugin/` are picked up on next session (or `/reload-plugins`). **No install step, no cache refresh, no shell alias needed during dev.**

Version-sync gotcha (tracked in beads `asana-cli-0a9`): keep `.claude-plugin/marketplace.json` `version` in lockstep with `claude-plugin/.claude-plugin/plugin.json` `version`. Both bump to `4.0.0` in the v4 release commit.

Orphan cleanup (tracked in beads `asana-cli-4t6`): `~/.claude/plugins/cache/asana-cli/` is a leftover from before the directory-source marketplace was configured. Safe to delete; not referenced by anything.

### File-by-file changes

#### `skills/using-asana-cli/SKILL.md`

- Add **Terminology** section near the top. Rule: user says "collaborator" → CLI means `--followers` / `--add-followers` / `--remove-followers`. First mention in a conversation: briefly acknowledge ("I'll use `--followers` — that's what the CLI and Asana API call them; same thing."). Silent on subsequent mentions.
- Add two rows to the operation-specific workflows table:
  - `Audit | references/AUDIT_TASKS.md | ...run tasks audit`
  - `Staged workflow | references/STAGED_WORKFLOW.md | ...create tasks that will be assigned later`
- Add **Strict resolution** section: names resolve exactly; ambiguity produces `USER_AMBIGUOUS` / `PROJECT_AMBIGUOUS` / `SECTION_AMBIGUOUS`; always pass `--json` on write commands so errors are structured and recoverable.
- Update top-level task-management examples to show new flag names and the `--json` pattern.

#### `skills/using-asana-cli/references/CREATE_TASK.md`

- Document `--force-unassigned`, `--skip-if-exists`, `--match-case`, `--include-completed`.
- Document explicit mirrors (`--assignee-email`, `--assignee-id`, `--project-id`, `--section-id`, `--followers-email`, `--followers-id`).
- **When resolution fails** section — exact error-envelope shape, candidate-parsing recipe (jq), auto-retry pattern when `candidates[0].email` is available.
- **What changed in v4** section — concrete before/after for partial-match removal.
- **Idempotent reruns** section — `--skip-if-exists` semantics, scope rules, the "skip is success (exit 0)" contract.

#### `skills/using-asana-cli/references/UPDATE_TASK.md`

- Document `--add-followers`, `--remove-followers`, `--unassign` with worked examples.
- Note: prefer `--add-followers` over bare `--followers`; both work, the former is the documented surface.
- Same strict-resolution notes + error-envelope handling as CREATE_TASK.md.

#### `skills/using-asana-cli/references/MOVE_TASK.md`

- Document `--project-id`, `--section-id` mirrors.
- Warn explicitly: partial project-name matches used to silently "work" and now error — the silent wrong-project move was arguably this command's worst pre-v4 footgun.

#### `skills/using-asana-cli/references/DELETE_TASK.md`

- Minor: note `delete` takes an ID directly, unaffected by strict mode.

#### NEW: `skills/using-asana-cli/references/AUDIT_TASKS.md`

- Full documentation for `tasks audit`: input modes, output shape (JSON + text), exit codes.
- Recipes:
  - "Verify a batch you just created" — `tasks audit --task-ids $(echo "$created_ids" | paste -sd,) --json`
  - "Dump state of an entire project section for review"
  - "Find tasks missing an assignee in a section"

#### NEW: `skills/using-asana-cli/references/STAGED_WORKFLOW.md`

The full recipe for the flow that fumbled. Sections:

- **When to use**: creating a batch of tasks where assignment notifications must not fire until the batch is complete.
- **Stage A — create unassigned, idempotent:**
  ```bash
  for rock in "Rock Alpha" "Rock Beta" "Rock Gamma"; do
    asana tasks create \
      -n "$rock" \
      -p "Project Alpha" \
      -s "Section A" \
      --due 2026-06-30 \
      --force-unassigned \
      --skip-if-exists \
      --json
  done
  ```
- **Stage B — assign by email:**
  ```bash
  asana tasks update "$TASK_ID" --assignee-email owner@example.com --json
  ```
- **Follower ensure/remove:**
  ```bash
  asana tasks update "$TASK_ID" --add-followers "a@example.com,b@example.com" --json
  asana tasks update "$TASK_ID" --remove-followers me@example.com --json
  ```
- **Verification:**
  ```bash
  asana tasks audit --task-ids "$TASK_IDS_CSV" --json > final-ledger.json
  ```
- **Terminology reminder**: if user says "collaborator," map to `--followers` / `--add-followers` / `--remove-followers`.
- **Common pitfalls**: Stage A + `--assignee` → `CONFLICTING_FLAGS`; Stage B when task already has a different assignee silently overwrites (Asana semantics, not ours) — use `tasks audit` first if that matters.

#### `skills/using-asana-cli/references/TROUBLESHOOTING.md`

- Add **Normalized error codes** section with one row per code (Section 5), including one-line remediation and example JSON envelope.
- Recipe: parsing the envelope in shell (`result=$(... --json 2>&1)` → `jq '.error'`).
- Recipe: "Did you mean" flow — for `*_NOT_FOUND` with non-empty `candidates[]`, suggest the top candidate.
- Recipe: exit-code-based branching for agents that don't want to parse JSON.

#### `agents/asana-task-manager.md`

- **Capabilities** list gains: `Audit task state`, `Staged workflows (create unassigned → assign later)`, `Add/remove followers`, `Unassign tasks`.
- New **Terminology** subsection with the collaborator-translation rule.
- Expanded **Guidelines**:
  - Always pass `--json` on write commands so errors are structured.
  - On `*_AMBIGUOUS`, read `candidates[]`, present to the user with emails, and retry with `--assignee-email` once disambiguated.
  - On `*_NOT_FOUND` with non-empty `candidates[]`, confirm the top suggestion with the user before retrying — don't silently guess.
  - For any batch of tasks, use `--skip-if-exists` so reruns after partial failure are safe.
  - After a batch of writes, verify with `tasks audit --task-ids ...` and surface the ledger.
- **Error Handling** refreshed to reference Section 5 codes, not the old "unknown flag" / "not found" heuristics.
- Explicit pointer to `STAGED_WORKFLOW.md` as the canonical recipe for "create many tasks; assign later; ensure specific followers; verify" flows.

### Plugin smoke test (ties to Section 7 pre-release plan)

1. Dev iteration happens in-place via the directory-source marketplace — no install step.
2. Before tagging v4: drive the full Stage-A → Stage-B → followers → audit flow through `asana-task-manager` using natural-language instructions ("create these rocks for Q2, keep them unassigned for now"). Agent must reach the terminal audit step without human intervention on any known footgun.
3. Deliberately trigger each error code (ambiguous user, unknown project, conflicting flags) and confirm the agent recovers per its guidelines.
4. Confirm the "collaborator → followers" terminology reminder fires exactly once per conversation.

---

## Summary table of all locked decisions

| Decision | Choice |
|---|---|
| Safety model | Full retrofit: strict resolution across all commands, no `--allow-ambiguous` escape hatch |
| Resolution rule | Case-insensitive exact match; multi-match = ambiguous; no partial matching |
| Staged workflow | Flags on existing commands (`--force-unassigned`), no `tasks workflow` subtree |
| Bulk ops | None. Composition via shell/agent loops; idempotency via `--skip-if-exists` |
| Flag strategy | Auto-detect on `-a` / `-p` / `-s` / `-f`, plus explicit `--*-email` / `--*-id` mirrors |
| Terminology | CLI stays "followers"; plugin translates "collaborators" → "followers" |
| `tasks audit` | New read-only command scoped by project/section or task-ids |
| Error shape | Normalized codes, structured JSON envelope, dedicated exit code 3 for resolution errors |
| Spec files | None (no bulk commands) |
| Markdown converter | Out of scope |
| Version | v4.0.0 (breaking); plugin.json and marketplace.json bump in lockstep |
| Dev workflow | Already live via directory-source marketplace; no install step needed |

---

## Related beads issues

- `asana-cli-93g` (open, P3) — concurrent section-task fetches in `listTasksWithSections`. Relevant to Section 4 (audit perf); share a bounded-concurrency helper.
- `asana-cli-4t6` (in_progress) — delete orphan plugin cache at `~/.claude/plugins/cache/asana-cli/`. Housekeeping from design.
- `asana-cli-0a9` (in_progress) — keep `marketplace.json` version in lockstep with `plugin.json`. Housekeeping from design.
