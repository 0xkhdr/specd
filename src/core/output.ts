// Redirectable stdout sink for raw (non-newline-terminated) CLI output.
// The CLI must NOT swap the global `process.stdout.write` for capture: the node:test reporter
// writes test events through that same function and flushes asynchronously, so swapping it
// silently drops test registrations. Direct writes go through this sink instead, which tests can
// redirect without ever touching `process.stdout.write`.
export const stdout = {
  write(s: string): void {
    process.stdout.write(s);
  },
};
