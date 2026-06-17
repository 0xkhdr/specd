# Criterion → Test Matrix (T2)

Maps every acceptance criterion R1.1–R6.3 to existing test func(s) in `internal/core`, or `UNMAPPED`.

| Criterion | Behavior | Test func(s) | Status |
|-----------|----------|--------------|--------|
| R1.1 | deps satisfied → in frontier | TestDagRunnableFrontier, TestNextRunnable, TestFrontierDetect | mapped |
| R1.2 | cycle → report + refuse frontier | TestDetectCycle, TestDetectCyclePath, TestGateDAG | mapped |
| R1.3 | wave: dep in earlier-or-equal wave | TestDagGroupWaves, TestDagWaveViolations, TestCriticalPathInvariant | mapped |
| R1.4 | incomplete dep → exclude dependents | TestNextRunnable, TestDagRunnableFrontier, TestOrphanDeps | mapped |
| R2.1 | open approval gate blocks advancement | — | **UNMAPPED** |
| R2.2 | flip lacks required evidence → reject | TestGateEvidence, TestGateAcceptanceCompleteWithoutPass | mapped |
| R2.3 | persist phase/gate/revision monotonically | TestSaveStateBumpsRevision, TestSaveLoadState | mapped |
| R2.4 | custom gates run in pipeline order | TestCustomGateRunner, TestCustomGatesDoNotAlterCoreGates | mapped (order assertion thin) |
| R3.1 | record evidence + timestamp | TestVerificationRecordCompat, TestGateEvidence | mapped (timestamp assertion thin) |
| R3.2 | verify cmd absent → reject unless --unverified | TestGateEvidence | **partial** (no --unverified path) |
| R3.3 | store telemetry annotations w/o computing | TestApplyTaskAnnotation, TestRollupTelemetry, TestAnnotationSeparatorRoundTrip | mapped |
| R4.1 | criterion no EARS pattern → flag | TestLintEars_stateMachine, TestLintEars_missingUserStory, TestMatchEars | mapped |
| R4.2 | accept five canonical EARS patterns | TestMatchEars_FormsAndGuards, TestMatchEars | mapped |
| R5.1 | sandbox configured → execute inside it | TestSelectRunnerNoneRegression, TestSelectContainerFailsClosedWithoutImage, TestSelectRunnerFailsClosedOnMissingIsolator | mapped |
| R5.2 | cmd fails → surface exit code + stderr verbatim | TestShRunnerUnchanged | mapped |
| R5.3 | sandbox none → still capture exit/output | TestShRunnerUnchanged, TestSelectRunnerNoneRegression | mapped |
| R6.1 | two writers → serialize, fail one deterministically | TestConcurrentSaveStateIsSerialized, TestSaveStateDetectsConcurrentWrite, TestCASConflict | mapped |
| R6.2 | stale lock → recover w/o corruption | TestWithSpecLockReclaimsStaleLock | mapped |
| R6.3 | state.json schema-valid after every write | TestSchemaConformance, TestSaveLoadState, TestSaveStateUnderLockPassesAssertion | mapped |

## Gaps (drive T3–T5)
- **R2.1 UNMAPPED** — no test asserts an open approval gate blocks `AdvancePhase`/approve. (T4)
- **R3.2 partial** — no test asserts `--unverified` bypass vs rejection when verify cmd absent. (T4)
- **R2.4 / R3.1 thin** — pipeline-order and evidence-timestamp assertions exist but weak; strengthen. (T4)
