const vscode = require('vscode');

const COMMANDS = {
  diagnostics: 'antigravity.getDiagnostics',
  startNewConversation: 'antigravity.startNewConversation',
  sendPrompt: 'antigravity.sendPromptToAgentPanel',
  setVisibleConversation: 'antigravity.setVisibleConversation'
};

class AntigravityCompatibility {
  constructor() {
    this._dispatch = Promise.resolve();
    this._commands = new Set();
  }

  async initialize() {
    this._commands = new Set(await vscode.commands.getCommands(true));
  }

  capabilities() {
    const conversationList = this._commands.has(COMMANDS.diagnostics);
    const conversationFocus = this._commands.has(COMMANDS.setVisibleConversation);
    const conversationSend = conversationFocus && this._commands.has(COMMANDS.sendPrompt);
    const conversationCreate = conversationList && this._commands.has(COMMANDS.startNewConversation);
    return {
      protocolVersion: 1,
      workspaceOpen: true,
      conversationList,
      conversationCreate,
      conversationFocus,
      conversationSend,
      conversationDirectSend: false,
      messageHistory: false,
      agentEvents: false,
      cancel: false,
      approvalEvents: false,
      approvalDecision: false,
      nativeFork: false,
      conversationCreateMode: conversationCreate ? 'command-fallback' : 'unsupported',
      conversationDispatchMode: conversationSend ? 'focus-then-send' : 'unsupported'
    };
  }

  async diagnostics() {
    const raw = await vscode.commands.executeCommand(COMMANDS.diagnostics);
    if (typeof raw !== 'string') throw new Error('getDiagnostics returned unexpected type');
    return JSON.parse(raw);
  }

  async listConversations() {
    const diag = await this.diagnostics();
    const entries = Array.isArray(diag.recentTrajectories) ? diag.recentTrajectories : [];
    return entries.map(item => ({
      id: String(item.googleAgentId || item.trajectoryId || ''),
      trajectoryId: String(item.trajectoryId || ''),
      title: String(item.summary || ''),
      lastStepIndex: Number.isFinite(item.lastStepIndex) ? item.lastStepIndex : 0,
      lastModifiedAt: String(item.lastModifiedTime || '')
    })).filter(item => item.id);
  }

  async createConversation() {
    if (!this.capabilities().conversationCreate) throw new Error('conversation creation unsupported');
    const before = new Set((await this.listConversations()).map(item => item.id));
    await vscode.commands.executeCommand(COMMANDS.startNewConversation);
    const deadline = Date.now() + 8000;
    while (Date.now() < deadline) {
      await delay(250);
      const current = await this.listConversations();
      const created = current.find(item => !before.has(item.id));
      if (created) return created;
    }
    throw new Error('new conversation was not visible in diagnostics before timeout');
  }

  async focusConversation(id) {
    if (!this.capabilities().conversationFocus) throw new Error('conversation focus unsupported');
    await vscode.commands.executeCommand(COMMANDS.setVisibleConversation, id);
  }

  async sendMessage(id, text) {
    if (!this.capabilities().conversationSend) throw new Error('conversation send unsupported');
    const operation = this._dispatch.then(async () => {
      await vscode.commands.executeCommand(COMMANDS.setVisibleConversation, id);
      await vscode.commands.executeCommand(COMMANDS.sendPrompt, text);
    });
    this._dispatch = operation.catch(() => {});
    return operation;
  }
}

function delay(ms) { return new Promise(resolve => setTimeout(resolve, ms)); }

module.exports = { AntigravityCompatibility };
