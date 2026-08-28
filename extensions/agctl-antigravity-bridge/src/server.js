const crypto = require('crypto');
const http = require('http');
const vscode = require('vscode');
const { AntigravityCompatibility } = require('./compatibility');
const { TrajectoryEventSource } = require('./events');
const { LanguageServerAdapter } = require('./languageServer');

const PROTOCOL_VERSION = 1;
const MAX_BODY = 1024 * 1024;

async function startBridge(options) {
  const workspaceFolders = (vscode.workspace.workspaceFolders || []).map(folder => folder.uri.fsPath);
  const languageServer = new LanguageServerAdapter({ workspaceFolders });
  const lsReady = await languageServer.initialize();
  if (!lsReady && languageServer.lastError) console.warn(`[agctl-bridge] agent mirror unavailable: ${languageServer.lastError}`);
  const eventSource = new TrajectoryEventSource(languageServer);
  const compatibility = new AntigravityCompatibility(eventSource);
  await compatibility.initialize();
  const server = http.createServer((req, res) => handle(req, res, options, compatibility));
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('bridge failed to resolve listen address');
  return { port: address.port, close: () => server.close() };
}

async function handle(req, res, options, compatibility) {
  try {
    if (!authorized(req, options.token)) return sendError(res, 401, 'UNAUTHORIZED', 'invalid bridge token');
    const method = req.method || 'GET';
    const url = new URL(req.url || '/', 'http://127.0.0.1');
    if (method === 'GET' && url.pathname === '/v1/health') {
      return sendData(res, { status: 'ok', instanceId: options.instanceId || '', bootNonce: options.bootNonce || '', pid: process.pid });
    }
    if (method === 'GET' && url.pathname === '/v1/capabilities') return sendData(res, compatibility.capabilities());
    if (method === 'GET' && url.pathname === '/v1/context') {
      return sendData(res, { instanceId: options.instanceId || '', bootNonce: options.bootNonce || '', pid: process.pid, workspaceFolders: (vscode.workspace.workspaceFolders || []).map(folder => folder.uri.fsPath) });
    }
    if (method === 'GET' && url.pathname === '/v1/conversations') return sendData(res, await compatibility.listConversations());
    if (method === 'POST' && url.pathname === '/v1/conversations') {
      await readJSON(req);
      return sendData(res, await compatibility.createConversation(), 201);
    }
    const eventsMatch = url.pathname.match(/^\/v1\/conversations\/([^/]+)\/events$/);
    if (method === 'GET' && eventsMatch) {
      const capabilities = compatibility.capabilities();
      if (!capabilities.agentEvents) return sendError(res, 503, 'CAPABILITY_UNAVAILABLE', 'agent events unavailable for this IDE instance');
      const after = parseAfter(url.searchParams.get('after'));
      return sendData(res, await compatibility.listEvents(decodeConversationId(eventsMatch[1]), after));
    }
    const focusMatch = url.pathname.match(/^\/v1\/conversations\/([^/]+)\/focus$/);
    if (method === 'POST' && focusMatch) {
      await readJSON(req);
      await compatibility.focusConversation(decodeConversationId(focusMatch[1]));
      return sendData(res, {});
    }
    const messageMatch = url.pathname.match(/^\/v1\/conversations\/([^/]+)\/messages$/);
    if (method === 'POST' && messageMatch) {
      const body = await readJSON(req);
      if (!body || typeof body.text !== 'string' || !body.text.trim()) return sendError(res, 400, 'INVALID_ARGUMENT', 'message text is required');
      await compatibility.sendMessage(decodeConversationId(messageMatch[1]), body.text);
      return sendData(res, {});
    }
    if (method === 'POST' && url.pathname === '/v1/workspace/open') {
      const body = await readJSON(req);
      if (!body || typeof body.path !== 'string' || !body.path.trim()) return sendError(res, 400, 'INVALID_ARGUMENT', 'workspace path is required');
      const target = vscode.Uri.file(body.path);
      setTimeout(() => vscode.commands.executeCommand('vscode.openFolder', target, false), 50);
      return sendData(res, { scheduled: true }, 202);
    }
    return sendError(res, 404, 'NOT_FOUND', url.pathname);
  } catch (error) {
    console.error('[agctl-bridge] request failed', error instanceof Error ? error.message : String(error));
    return sendError(res, 500, 'BRIDGE_ERROR', error instanceof Error ? error.message : String(error));
  }
}

function parseAfter(value) {
  if (value == null || value === '') return 0;
  if (!/^\d+$/.test(value)) throw new Error('after must be a non-negative integer');
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 0) throw new Error('after is outside safe integer range');
  return parsed;
}

function decodeConversationId(value) {
  const id = decodeURIComponent(value);
  if (!id || id.length > 512 || /[\x00-\x1f/\\]/.test(id)) throw new Error('invalid conversation id');
  return id;
}

function authorized(req, expected) {
  const value = String(req.headers.authorization || '');
  const prefix = 'Bearer ';
  if (!value.startsWith(prefix)) return false;
  const supplied = Buffer.from(value.slice(prefix.length));
  const wanted = Buffer.from(String(expected || ''));
  return supplied.length === wanted.length && wanted.length > 0 && crypto.timingSafeEqual(supplied, wanted);
}

async function readJSON(req) {
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > MAX_BODY) throw new Error('request body too large');
    chunks.push(chunk);
  }
  if (size === 0) return {};
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

function sendData(res, data, status = 200) {
  const payload = JSON.stringify({ protocolVersion: PROTOCOL_VERSION, ok: true, data });
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(payload), 'cache-control': 'no-store' });
  res.end(payload);
}

function sendError(res, status, code, message) {
  const payload = JSON.stringify({ protocolVersion: PROTOCOL_VERSION, ok: false, error: { code, message } });
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(payload), 'cache-control': 'no-store' });
  res.end(payload);
}

module.exports = { startBridge, parseAfter, decodeConversationId };
