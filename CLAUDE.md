# CLAUDE.md — Contributor Guidelines

This file is a quick reference for developers and coding agents contributing to the `specd` repository.

## Commands

### Build & Compilation
```sh
npm run build          # Compiles TypeScript to dist/ and copies templates
```

### Running Tests
```sh
npm test               # Runs the full test suite (92 tests covering gates, DAGs, concurrency)
```

### Running from Source
```sh
node --import tsx src/cli.ts <command>  # Run CLI commands directly without building
```

---

## Code Style & Invariants

All contributions must respect these core architectural constraints:

1. **Zero Runtime Dependencies**: The `dependencies` object in `package.json` must remain empty. Use only Node.js built-ins and devDependencies (like `tsx` and `@types/node`).
2. **Durable & Atomic File Writes**: Never use `fs.writeFileSync` directly for mutable state (`state.json` or `tasks.md`). You must use `atomicWrite` (defined in [io.ts](file:///var/www/html/rai/up/specd/src/core/io.ts)) which writes to a temporary file, forces disk synchronization (`fsync`), and renames it atomically.
3. **Optimistic Concurrency (CAS)**: State mutations must load `state.json`, verify that the `revision` matches, and increment the revision count on write.
4. **Reentrant Advisory Locks**: Mutating commands must acquire the spec-specific advisory lock using `withSpecLock` (defined in [lock.ts](file:///var/www/html/rai/up/specd/src/core/lock.ts)).
5. **Round-Trip Parser Stability**: The custom tasks parser in [tasksParser.ts](file:///var/www/html/rai/up/specd/src/core/tasksParser.ts) must maintain 100% round-trip byte equivalence. Parsing a tasks markdown file and serializing it back must produce the identical byte sequence.
6. **Exit Code Semantics**:
   - `0`: Operation succeeded or validation checks passed.
   - `1`: Validation gate or check failed (e.g., EARS syntax, design headers, DAG cycle).
   - `2`: Usage error / CLI arguments error.
   - `3`: Root `.specd/` directory or specified spec slug not found.

---

## Project Structure

- **[src/cli.ts](file:///var/www/html/rai/up/specd/src/cli.ts)**: Command line entry point and dispatch routing.
- **[src/commands/](file:///var/www/html/rai/up/specd/src/commands)**: Contains CLI command handlers (one file per command, e.g., `init.ts`, `check.ts`, `task.ts`).
- **[src/core/](file:///var/www/html/rai/up/specd/src/core)**: Core domain logic (DAG engine, EARS parser, locks, state management, report generators).
- **[src/templates/](file:///var/www/html/rai/up/specd/src/templates)**: Default configuration and steering documents copied to `dist/templates/` on build.
- **[test/](file:///var/www/html/rai/up/specd/test)**: Comprehensive test suite split into core logic tests, parser tests, gate tests, and full lifecycle end-to-end scenarios.
