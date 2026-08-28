const fs = require('fs');
const os = require('os');
const path = require('path');
const vscode = require('vscode');
const { startBridge } = require('./server');

async function activate(context) {
  const token = String(process.env.AGCTL_BRIDGE_TOKEN || '').trim();
  if (!token) {
    console.warn('[agctl-bridge] AGCTL_BRIDGE_TOKEN is missing; bridge stays disabled');
    return;
  }
  const instanceId = String(process.env.AGCTL_INSTANCE_ID || '').trim();
  const bootNonce = String(process.env.AGCTL_BOOT_NONCE || '').trim();
  const registryRoot = String(process.env.AGCTL_BRIDGE_REGISTRY || '').trim();
  const bridge = await startBridge({ token, instanceId, bootNonce });
  context.subscriptions.push({ dispose: () => bridge.close() });

  if (registryRoot) {
    fs.mkdirSync(registryRoot, { recursive: true });
    const fileName = `${bootNonce || process.pid}.json`;
    const target = path.join(registryRoot, fileName);
    const temp = `${target}.${process.pid}.tmp`;
    const folders = (vscode.workspace.workspaceFolders || []).map(folder => folder.uri.fsPath);
    const registration = {
      protocolVersion: 1,
      instanceId,
      bootNonce,
      pid: process.pid,
      port: bridge.port,
      workspaceFolders: folders,
      startedAt: new Date().toISOString()
    };
    fs.writeFileSync(temp, JSON.stringify(registration, null, 2), { mode: 0o600 });
    fs.renameSync(temp, target);
    context.subscriptions.push({
      dispose: () => {
        try { fs.unlinkSync(target); } catch (_) {}
      }
    });
  }

  console.log(`[agctl-bridge] listening on 127.0.0.1:${bridge.port} pid=${process.pid} host=${os.hostname()}`);
}

function deactivate() {}

module.exports = { activate, deactivate };
