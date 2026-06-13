// Centralized exit-code contract (SPEC §5.1).
//   0 success/valid · 1 validation/gate failure · 2 usage error · 3 not found

export type ExitCode = 0 | 1 | 2 | 3;

export const EXIT = {
  OK: 0,
  GATE: 1,
  USAGE: 2,
  NOT_FOUND: 3,
} as const;

/** Error carrying an explicit process exit code. Thrown by commands/core, caught in cli.ts. */
export class SpecdError extends Error {
  readonly code: ExitCode;
  constructor(code: ExitCode, message: string) {
    super(message);
    this.name = "SpecdError";
    this.code = code;
  }
}

export const usageError = (msg: string) => new SpecdError(EXIT.USAGE, msg);
export const gateError = (msg: string) => new SpecdError(EXIT.GATE, msg);
export const notFoundError = (msg: string) => new SpecdError(EXIT.NOT_FOUND, msg);
