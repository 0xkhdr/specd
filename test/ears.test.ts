import { test } from "node:test";
import assert from "node:assert/strict";
import { matchEars, lintEars } from "../src/core/ears.js";

test("matchEars classifies each pattern", () => {
  assert.equal(matchEars("THE SYSTEM SHALL log events"), "ubiquitous");
  assert.equal(matchEars("WHEN x happens THE SYSTEM SHALL respond"), "event-driven");
  assert.equal(matchEars("WHILE running THE SYSTEM SHALL poll"), "state-driven");
  assert.equal(matchEars("WHERE feature on THE SYSTEM SHALL enable"), "optional-feature");
  assert.equal(matchEars("IF bad THEN THE SYSTEM SHALL reject"), "unwanted");
  assert.equal(matchEars("the system does stuff sometimes"), null);
});

const VALID = `# Requirements — X

## Requirement 1 — name
**User story:** As a user, I want x, so that y.

**Acceptance criteria:**
1. WHEN a happens THE SYSTEM SHALL b
2. THE SYSTEM SHALL c
`;

test("valid requirements lint clean", () => {
  assert.deepEqual(lintEars(VALID), []);
});

test("non-EARS criterion flagged with line", () => {
  const bad = VALID.replace("WHEN a happens THE SYSTEM SHALL b", "it should probably work");
  const issues = lintEars(bad);
  assert.ok(issues.some((i) => /EARS/.test(i.message)));
});

test("missing user story flagged", () => {
  const bad = VALID.replace("**User story:** As a user, I want x, so that y.\n", "");
  assert.ok(lintEars(bad).some((i) => /User story/.test(i.message)));
});

test("requirement without criteria flagged", () => {
  const bad = `# Requirements — X\n\n## Requirement 1 — n\n**User story:** As a, I want b, so that c.\n`;
  assert.ok(lintEars(bad).some((i) => /no acceptance criteria/.test(i.message)));
});
