// Centralized UI/logging module for specd.
import { stdout } from "./output.js";

export type LogLevel = "info" | "success" | "warn" | "error" | "step";

export interface LogEntry {
  level: LogLevel;
  message: string;
  timestamp: string; // ISO 8601
}

export function isJsonMode(): boolean {
  return process.env.SPECd_JSON === "1" || process.env.SPECd_JSON === "true" || !!(globalThis as any).specdJsonMode;
}

function useColor(): boolean {
  if (process.env.NO_COLOR && process.env.NO_COLOR !== "0") return false;
  return true;
}

const colors = {
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  red: "\x1b[31m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  blue: "\x1b[34m",
  gray: "\x1b[90m",
  cyan: "\x1b[36m",
};

function colorize(color: keyof typeof colors, text: string): string {
  if (!useColor()) return text;
  return `${colors[color]}${text}${colors.reset}`;
}

export function json(level: LogLevel, message: string): LogEntry {
  return {
    level,
    message,
    timestamp: new Date().toISOString(),
  };
}

function printJson(level: LogLevel, message: string, stream: "stdout" | "stderr" = "stdout"): void {
  const entryStr = JSON.stringify(json(level, message));
  if (stream === "stderr") {
    console.error(entryStr);
  } else {
    stdout.write(entryStr + "\n");
  }
}

export function info(msg: string): void {
  if (isJsonMode()) {
    printJson("info", msg);
    return;
  }
  stdout.write(`${colorize("blue", "info")}  ${msg}\n`);
}

export function success(msg: string): void {
  if (isJsonMode()) {
    printJson("success", msg);
    return;
  }
  stdout.write(`${colorize("green", "✓")} ${msg}\n`);
}

export function warn(msg: string): void {
  if (isJsonMode()) {
    printJson("warn", msg);
    return;
  }
  stdout.write(`${colorize("yellow", "warn")}  ${msg}\n`);
}

export function error(msg: string): void {
  if (isJsonMode()) {
    printJson("error", msg, "stderr");
    return;
  }
  console.error(`${colorize("red", "error")}: ${msg}`);
}

let lastStepLabel = "";
let lastStepActive = false;

export function step(label: string, status?: "pending" | "done" | "failed" | string): void {
  if (isJsonMode()) {
    printJson("step", status ? `${label} (${status})` : label);
    return;
  }

  const isInteractive = process.stdout.isTTY && !process.env.CI;

  if (status === "pending") {
    if (lastStepActive && isInteractive) {
      stdout.write("\r\x1b[K");
    }
    lastStepLabel = label;
    lastStepActive = true;
    if (isInteractive) {
      stdout.write(`[specd] 🔍 ${label}...`);
    } else {
      stdout.write(`[specd] 🔍 ${label}...\n`);
    }
  } else if (status === "done" || status === "success") {
    if (lastStepActive && lastStepLabel === label && isInteractive) {
      stdout.write(`\r\x1b[K[specd] 🔍 ${label}...      ${colorize("green", "✓")}\n`);
    } else {
      stdout.write(`[specd] 🔍 ${label}...      ${colorize("green", "✓")}\n`);
    }
    lastStepActive = false;
  } else if (status === "failed" || status === "error") {
    if (lastStepActive && lastStepLabel === label && isInteractive) {
      stdout.write(`\r\x1b[K[specd] 🔍 ${label}...      ${colorize("red", "❌")}\n`);
    } else {
      stdout.write(`[specd] 🔍 ${label}...      ${colorize("red", "❌")}\n`);
    }
    lastStepActive = false;
  } else if (status !== undefined) {
    if (lastStepActive && lastStepLabel === label && isInteractive) {
      stdout.write(`\r\x1b[K[specd] 🔍 ${label}...      ${colorize("green", "✓")} ${status}\n`);
    } else {
      stdout.write(`[specd] 🔍 ${label}...      ${colorize("green", "✓")} ${status}\n`);
    }
    lastStepActive = false;
  } else {
    if (lastStepActive && isInteractive) {
      stdout.write("\n");
    }
    stdout.write(`[specd] 🔍 ${label}\n`);
    lastStepActive = false;
  }
}

export function header(title: string): void {
  if (isJsonMode()) return;
  stdout.write(`\n${colorize("bold", title.toUpperCase())}\n`);
}

export function divider(): void {
  if (isJsonMode()) return;
  stdout.write(`${colorize("gray", "─".repeat(50))}\n`);
}
