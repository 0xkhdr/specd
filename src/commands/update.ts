// `specd update [--force]` — self-update command.
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import type { Args } from "../cli.js";
import { EXIT, SpecdError } from "../core/exit.js";
import { info, success, error, step } from "../core/ui.js";

interface GitHubRelease {
  tag_name: string;
}

async function getLatestGitHubRelease(): Promise<string | null> {
  try {
    const res = await fetch("https://api.github.com/repos/0xkhdr/specd/releases/latest", {
      headers: { "User-Agent": "specd-cli-updater" },
      signal: AbortSignal.timeout(5000) // 5s timeout
    });
    if (res.ok) {
      const data = await res.json() as GitHubRelease;
      if (data && data.tag_name) return data.tag_name;
    }
  } catch {
    // Fall back to git ls-remote on network error/timeout
  }
  return null;
}

function getLatestCommitHash(): string | null {
  try {
    const stdout = execSync("git ls-remote https://github.com/0xkhdr/specd.git refs/heads/main", {
      stdio: "pipe",
      timeout: 5000
    }).toString();
    const parts = stdout.trim().split(/\s+/);
    if (parts[0]) return parts[0];
  } catch {
    // git ls-remote failed
  }
  return null;
}

export async function run(args: Args): Promise<number> {
  const force = args.flags.force === true;

  // Resolve repository root relative to this file
  const here = dirname(fileURLToPath(import.meta.url));
  const repoRoot = join(here, "..", "..");

  let currentVersion = "v0.0.0";
  const pkgPath = join(repoRoot, "package.json");
  try {
    const pkg = JSON.parse(readFileSync(pkgPath, "utf8")) as { version: string };
    currentVersion = `v${pkg.version}`;
  } catch (err) {
    throw new SpecdError(EXIT.GATE, `Could not read package.json at ${pkgPath}`);
  }

  let currentCommit = "";
  try {
    currentCommit = execSync("git rev-parse HEAD", { cwd: repoRoot, stdio: "pipe" }).toString().trim();
  } catch {
    // Not a git repo or git not installed/available
  }

  step("Checking for updates", "pending");

  let latestVersion: string | null = null;
  let latestCommit: string | null = null;

  try {
    latestVersion = await getLatestGitHubRelease();
    if (!latestVersion) {
      latestCommit = getLatestCommitHash();
    }
  } catch {
    // Catch-all for network issues
  }

  if (!latestVersion && !latestCommit) {
    step("Checking for updates", "failed");
    throw new SpecdError(EXIT.GATE, "Unable to check for updates. You appear to be offline or the update server is unreachable.");
  }

  let needsUpdate = false;
  let target = "";

  if (latestVersion) {
    target = latestVersion;
    needsUpdate = latestVersion !== currentVersion;
  } else if (latestCommit) {
    target = latestCommit;
    needsUpdate = latestCommit !== currentCommit;
  }

  if (!needsUpdate && !force) {
    step("Checking for updates", "done");
    success(`Already up to date (${currentVersion})`);
    return 0;
  }

  step("Checking for updates", target);

  // Save current ref for rollback
  let originalRef = "";
  if (currentCommit) {
    originalRef = currentCommit;
  } else {
    originalRef = currentVersion;
  }

  step("Downloading updates", "pending");
  try {
    // If not a git repo, we can't update via git pull
    if (!currentCommit) {
      throw new Error("Local directory is not a git repository. Cannot update in-place.");
    }
    execSync("git fetch origin", { cwd: repoRoot, stdio: "pipe" });
    if (latestVersion) {
      execSync(`git checkout ${latestVersion}`, { cwd: repoRoot, stdio: "pipe" });
    } else if (latestCommit) {
      execSync("git pull origin main", { cwd: repoRoot, stdio: "pipe" });
    }
    step("Downloading updates", "done");
  } catch (err) {
    step("Downloading updates", "failed");
    throw new SpecdError(EXIT.GATE, `Failed to download update: ${err instanceof Error ? err.message : String(err)}`);
  }

  step("Building from source", "pending");
  try {
    execSync("npm install", { cwd: repoRoot, stdio: "pipe" });
    execSync("npm run build", { cwd: repoRoot, stdio: "pipe" });
    step("Building from source", "done");
  } catch (err) {
    step("Building from source", "failed");
    info(`Rolling back to ${originalRef.slice(0, 7)}...`);
    try {
      execSync(`git checkout ${originalRef}`, { cwd: repoRoot, stdio: "pipe" });
      execSync("npm install && npm run build", { cwd: repoRoot, stdio: "pipe" });
      info("Rollback completed successfully.");
    } catch (rollbackErr) {
      error(`Rollback failed: ${rollbackErr instanceof Error ? rollbackErr.message : String(rollbackErr)}`);
    }
    throw new SpecdError(EXIT.GATE, `Update failed during build. Rolled back. Error: ${err instanceof Error ? err.message : String(err)}`);
  }

  let updatedVersion = currentVersion;
  try {
    const pkg = JSON.parse(readFileSync(pkgPath, "utf8")) as { version: string };
    updatedVersion = `v${pkg.version}`;
  } catch {}

  success(`Update complete! specd ${updatedVersion}`);
  return 0;
}
