'use strict';

const childProcess = require('child_process');
const crypto = require('crypto');
const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');
const util = require('util');

const execFile = util.promisify(childProcess.execFile);
const SERVICE = '/exa.language_server_pb.LanguageServerService/';
const MAX_RESPONSE = 8 * 1024 * 1024;

class LanguageServerAdapter {
  constructor(options = {}) {
    this._workspaceFolders = Array.isArray(options.workspaceFolders) ? options.workspaceFolders.filter(Boolean) : [];
    this._execFile = options.execFile || execFile;
    this._request = options.request || requestRPC;
    this._connection = null;
    this.lastError = '';
  }

  get ready() { return Boolean(this._connection); }

  async initialize() {
    try {
      const processInfo = await discoverLanguageServerProcess(this._workspaceFolders, this._execFile);
      const ports = await discoverListeningPorts(processInfo.pid, processInfo.extensionServerPort, this._execFile);
      for (const tls of [true, false]) {
        for (const port of ports) {
          try {
            const connection = { port, csrfToken: processInfo.csrfToken, tls };
            await this._request(connection, 'GetUserStatus', {}, 3000);
            this._connection = connection;
            this.lastError = '';
            return true;
          } catch (_) {}
        }
      }
      throw new Error('no CSRF-authenticated ConnectRPC endpoint found for matched language_server process');
    } catch (error) {
      this._connection = null;
      this.lastError = error instanceof Error ? error.message : String(error);
      return false;
    }
  }

  async getTrajectory(conversationId) {
    if (!this._connection) throw new Error('Antigravity Language Server is not ready');
    const id = validateConversationId(conversationId);
    return this._request(this._connection, 'GetCascadeTrajectory', { cascadeId: id }, 7000);
  }
}

async function discoverLanguageServerProcess(workspaceFolders, runner = execFile) {
  const candidates = process.platform === 'win32'
    ? await windowsLanguageServerProcesses(runner)
    : await unixLanguageServerProcesses(runner);
  const valid = candidates.filter(item => item.pid > 0 && item.csrfToken);
  if (valid.length === 0) throw new Error('no language_server process with --csrf_token found');
  const hints = workspaceFolders.map(workspaceProcessHint).filter(Boolean);
  let matched = valid;
  if (hints.length > 0) {
    const byWorkspace = valid.filter(item => hints.some(hint => normalizedCommandLine(item.commandLine).includes(hint)));
    if (byWorkspace.length > 0) matched = byWorkspace;
  }
  if (matched.length !== 1) {
    throw new Error(`language_server identity is ambiguous (${matched.length} candidates)`);
  }
  return matched[0];
}

async function windowsLanguageServerProcesses(runner) {
  const script = "Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match 'language_server' -and $_.CommandLine -match 'csrf_token' } | ForEach-Object { $_.ProcessId.ToString() + '|' + $_.CommandLine }";
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  const { stdout } = await runner('powershell.exe', ['-NoProfile', '-NonInteractive', '-EncodedCommand', encoded], { encoding: 'utf8', timeout: 10000, windowsHide: true });
  return String(stdout || '').split(/\r?\n/).map(parseWindowsProcessLine).filter(Boolean);
}

async function unixLanguageServerProcesses(runner) {
  const { stdout } = await runner('ps', ['-eo', 'pid=,args='], { encoding: 'utf8', timeout: 5000 });
  return String(stdout || '').split(/\r?\n/).map(line => {
    const match = line.match(/^\s*(\d+)\s+(.+)$/);
    if (!match || !/language_server/i.test(match[2]) || !/--csrf_token(?:=|\s)/.test(match[2])) return null;
    return processRecord(Number(match[1]), match[2]);
  }).filter(Boolean);
}

function parseWindowsProcessLine(line) {
  const separator = String(line || '').indexOf('|');
  if (separator <= 0) return null;
  const pid = Number(String(line).slice(0, separator).trim());
  const commandLine = String(line).slice(separator + 1).trim();
  if (!Number.isSafeInteger(pid) || pid <= 0 || !commandLine) return null;
  return processRecord(pid, commandLine);
}

function processRecord(pid, commandLine) {
  return {
    pid,
    commandLine,
    csrfToken: extractArg(commandLine, 'csrf_token') || '',
    extensionServerPort: Number(extractArg(commandLine, 'extension_server_port') || 0)
  };
}

function extractArg(commandLine, name) {
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const equal = String(commandLine || '').match(new RegExp(`--${escaped}=([^\\s\"]+)`));
  if (equal) return equal[1];
  const spaced = String(commandLine || '').match(new RegExp(`--${escaped}\\s+([^\\s\"]+)`));
  return spaced ? spaced[1] : null;
}

function workspaceProcessHint(folder) {
  const normalized = String(folder || '').replace(/\\/g, '/').replace(/\/+$/, '');
  if (!normalized) return '';
  const parts = normalized.split('/').filter(Boolean).slice(-2);
  return parts.join('_').replace(/[-.\s:]/g, '_').toLowerCase();
}

function normalizedCommandLine(commandLine) {
  return String(commandLine || '').replace(/\\/g, '/').replace(/[-.\s:/]+/g, '_').toLowerCase();
}

async function discoverListeningPorts(pid, excludedPort, runner = execFile) {
  let ports = [];
  if (process.platform === 'win32') {
    const { stdout } = await runner('netstat.exe', ['-aon', '-p', 'tcp'], { encoding: 'utf8', timeout: 5000, windowsHide: true });
    ports = parseNetstatPorts(stdout, pid);
  } else {
    try {
      const { stdout } = await runner('ss', ['-ltnp'], { encoding: 'utf8', timeout: 5000 });
      ports = parseSSPorts(stdout, pid);
    } catch (_) {
      const { stdout } = await runner('netstat', ['-ltnp'], { encoding: 'utf8', timeout: 5000 });
      ports = parseSSPorts(stdout, pid);
    }
  }
  return [...new Set(ports)].filter(port => port > 0 && port <= 65535 && port !== excludedPort).sort((a, b) => a - b);
}

function parseNetstatPorts(output, pid) {
  const wanted = String(pid);
  const out = [];
  for (const line of String(output || '').split(/\r?\n/)) {
    if (!/LISTENING/i.test(line)) continue;
    const fields = line.trim().split(/\s+/);
    if (fields.length < 5 || fields[fields.length - 1] !== wanted) continue;
    const local = fields[1] || '';
    const match = local.match(/(?:127\.0\.0\.1|\[::1\]|::1):(\d+)$/);
    if (match) out.push(Number(match[1]));
  }
  return out;
}

function parseSSPorts(output, pid) {
  const out = [];
  const pidPattern = new RegExp(`pid=${pid}(?:,|\\))`);
  for (const line of String(output || '').split(/\r?\n/)) {
    if (!pidPattern.test(line)) continue;
    const match = line.match(/(?:127\.0\.0\.1|\[::1\]|::1):(\d+)/);
    if (match) out.push(Number(match[1]));
  }
  return out;
}

function requestRPC(connection, method, payload, timeoutMs) {
  if (!connection || !Number.isInteger(connection.port) || connection.port <= 0 || connection.port > 65535) return Promise.reject(new Error('invalid LS connection port'));
  if (!connection.csrfToken) return Promise.reject(new Error('missing LS CSRF token'));
  if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(method)) return Promise.reject(new Error('invalid LS method'));
  const transport = connection.tls ? https : http;
  const body = JSON.stringify(payload || {});
  return new Promise((resolve, reject) => {
    const req = transport.request({
      host: '127.0.0.1',
      port: connection.port,
      path: SERVICE + method,
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'content-length': Buffer.byteLength(body),
        'x-codeium-csrf-token': connection.csrfToken
      },
      timeout: Math.max(250, Math.min(Number(timeoutMs) || 5000, 15000)),
      ...(connection.tls ? { rejectUnauthorized: false } : {})
    }, res => {
      const chunks = [];
      let size = 0;
      res.on('data', chunk => {
        size += chunk.length;
        if (size > MAX_RESPONSE) {
          req.destroy(new Error('LS response exceeds limit'));
          return;
        }
        chunks.push(chunk);
      });
      res.on('end', () => {
        const text = Buffer.concat(chunks).toString('utf8');
        if (res.statusCode !== 200) return reject(new Error(`LS ${method} returned HTTP ${res.statusCode}`));
        try { resolve(text ? JSON.parse(text) : {}); }
        catch (error) { reject(new Error(`LS ${method} returned invalid JSON: ${error.message}`)); }
      });
    });
    req.once('timeout', () => req.destroy(new Error(`LS ${method} timed out`)));
    req.once('error', reject);
    req.end(body);
  });
}

function validateConversationId(value) {
  const id = String(value || '').trim();
  if (!id || id.length > 512 || /[\x00-\x1f/\\]/.test(id)) throw new Error('invalid conversation id');
  return id;
}

module.exports = {
  LanguageServerAdapter,
  discoverLanguageServerProcess,
  discoverListeningPorts,
  extractArg,
  parseNetstatPorts,
  parseSSPorts,
  workspaceProcessHint,
  validateConversationId
};
