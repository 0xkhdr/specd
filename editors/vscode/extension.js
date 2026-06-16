// specd Live Dashboard — minimal VS Code extension.
//
// It starts the read-only `specd serve` server for the chosen spec on a loopback
// port and embeds the dashboard in a webview. The extension is read-only: it
// only ever launches `specd serve` (which exposes no mutating routes) and never
// writes spec state itself. It is a separate package from the Go binary and adds
// no dependency to it.

const vscode = require("vscode");
const cp = require("child_process");
const http = require("http");

let serverProc = null;

function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand("specd.openDashboard", () => openDashboard(context))
  );
}

async function openDashboard(context) {
  const folder = vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0];
  if (!folder) {
    vscode.window.showErrorMessage("specd: open a workspace folder first.");
    return;
  }
  const root = folder.uri.fsPath;

  const slug = await pickSpec(root);
  if (!slug) return;

  const cfg = vscode.workspace.getConfiguration("specd");
  const bin = cfg.get("binaryPath", "specd");
  const port = cfg.get("servePort", 8765);
  const addr = `127.0.0.1:${port}`;
  const url = `http://${addr}/`;

  // (Re)launch the read-only server.
  if (serverProc) {
    serverProc.kill();
    serverProc = null;
  }
  serverProc = cp.spawn(bin, ["serve", slug, "--addr", addr], { cwd: root });
  serverProc.on("error", (e) =>
    vscode.window.showErrorMessage(`specd serve failed: ${e.message}`)
  );
  context.subscriptions.push({ dispose: () => serverProc && serverProc.kill() });

  await waitForServer(url, 5000);

  const panel = vscode.window.createWebviewPanel(
    "specdDashboard",
    `specd: ${slug}`,
    vscode.ViewColumn.One,
    { enableScripts: true, retainContextWhenHidden: true }
  );
  // Embed the served dashboard. The server is loopback + read-only.
  panel.webview.html = `<!DOCTYPE html><html><head><meta charset="utf-8">
    <style>html,body,iframe{margin:0;height:100%;width:100%;border:0}</style></head>
    <body><iframe src="${url}"></iframe></body></html>`;
  panel.onDidDispose(() => {
    if (serverProc) {
      serverProc.kill();
      serverProc = null;
    }
  });
}

// pickSpec lists spec slugs under .specd/specs and asks the user to choose.
async function pickSpec(root) {
  const fs = require("fs");
  const path = require("path");
  const dir = path.join(root, ".specd", "specs");
  let slugs = [];
  try {
    slugs = fs
      .readdirSync(dir, { withFileTypes: true })
      .filter((e) => e.isDirectory() && fs.existsSync(path.join(dir, e.name, "state.json")))
      .map((e) => e.name);
  } catch (_) {
    /* no specs */
  }
  if (slugs.length === 0) {
    vscode.window.showErrorMessage("specd: no specs found under .specd/specs.");
    return null;
  }
  if (slugs.length === 1) return slugs[0];
  return vscode.window.showQuickPick(slugs, { placeHolder: "Select a spec" });
}

// waitForServer polls the dashboard URL until it responds or the deadline passes.
function waitForServer(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve) => {
    const tick = () => {
      const req = http.get(url, (res) => {
        res.destroy();
        resolve(true);
      });
      req.on("error", () => {
        if (Date.now() > deadline) return resolve(false);
        setTimeout(tick, 150);
      });
    };
    tick();
  });
}

function deactivate() {
  if (serverProc) serverProc.kill();
}

module.exports = { activate, deactivate };
