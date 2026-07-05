# 0001 Wave 0 Hygiene

Status: accepted

Context:
Wave 0 found repository drift between documented local gates, CI wiring, and files present in this repo.

Decision:
Keep the root Makefile absent. Local and CI gates run direct commands or scripts under `scripts/`. Release metadata is injected into `internal/version`, not `main`.

Consequences:
Docs must name only scripts that exist. Release and CI jobs must use direct Go/script commands so drift is visible in this repository.
