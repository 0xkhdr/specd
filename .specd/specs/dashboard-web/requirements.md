# Requirements — Dashboard Decoupling from VS Code

## Introduction
specd's dashboard already lives in Go: `specd serve <slug>` renders the same self-contained HTML
as `specd report --format html` at `GET /` plus a JSON `GET /api/report`, bound to loopback. The
VS Code extension in `editors/vscode/` adds nothing but a wrapper — it spawns `specd serve` and
embeds the served URL in a webview iframe. This feature removes that VS Code coupling and makes
the dashboard a first-class, browser-native experience: a multi-spec index, live updates over the
existing SSE frontier stream instead of meta-refresh polling, and responsive layout — all served
by the Go binary with zero Node build step. The dashboard stays strictly read-only; it never
mutates spec state. Existing `specd serve` routes remain backward compatible.

## Requirement 1 — Remove the VS Code extension coupling
**User story:** As a maintainer, I want the dashboard decoupled from VS Code, so that no client is
privileged and the wrapper is not a maintenance burden.

**Acceptance criteria:**
1. THE SYSTEM SHALL remove the `editors/vscode/` extension from the repository
2. THE SYSTEM SHALL document the migration so existing VS Code users open the dashboard in a browser instead
3. THE SYSTEM SHALL preserve the existing `specd serve` read-only HTTP behaviour as the supported replacement

## Requirement 2 — Browser-native multi-spec dashboard
**User story:** As a user, I want to open the dashboard in any browser and see all my specs, so
that I am not limited to one editor or one spec at a time.

**Acceptance criteria:**
1. WHEN a browser requests the dashboard root THE SYSTEM SHALL serve a spec index listing every spec under the project
2. WHEN a user selects a spec THE SYSTEM SHALL render that spec's live report HTML
3. THE SYSTEM SHALL serve the dashboard from the Go binary with no Node or external build step
4. IF a requested spec does not exist THEN THE SYSTEM SHALL respond with HTTP 404

## Requirement 3 — Read-only invariant
**User story:** As a security-conscious operator, I want a guarantee the dashboard cannot change
state, so that exposing it carries no write risk.

**Acceptance criteria:**
1. THE SYSTEM SHALL reject any non-GET request to a dashboard route with HTTP 405
2. THE SYSTEM SHALL read spec data from state.json and artifacts only and never write spec state
3. THE SYSTEM SHALL rebuild every response from disk per request so the view always reflects current state

## Requirement 4 — Live updates without polling
**User story:** As a user watching a run, I want the dashboard to update as the frontier changes,
so that I see progress without manual refresh.

**Acceptance criteria:**
1. WHEN the runnable frontier of a served spec changes THE SYSTEM SHALL push an update to connected browsers over the existing SSE frontier stream
2. WHILE a browser is connected to the event stream THE SYSTEM SHALL deliver an initial snapshot followed by deltas
3. THE SYSTEM SHALL render dashboard updates programmatically from state.json with no LLM call

## Requirement 5 — Network exposure safety
**User story:** As an operator, I want safe defaults when serving the dashboard, so that spec
contents are not accidentally exposed off-host.

**Acceptance criteria:**
1. THE SYSTEM SHALL bind the dashboard to loopback (127.0.0.1) by default
2. WHERE a non-loopback bind address is supplied THE SYSTEM SHALL require it to be set explicitly by the operator
3. IF the server cannot bind the requested address THEN THE SYSTEM SHALL exit with a gate error

## Requirement 6 — Responsive cross-browser layout
**User story:** As a user on desktop, tablet, or phone, I want the dashboard to be usable on my
screen, so that I can check spec status from any device.

**Acceptance criteria:**
1. THE SYSTEM SHALL render a responsive layout that adapts to desktop, tablet, and mobile viewport widths
2. THE SYSTEM SHALL keep the dashboard HTML self-contained with inlined styles and no external asset fetches
3. THE SYSTEM SHALL render correctly on current Chromium, Firefox, and WebKit browsers
