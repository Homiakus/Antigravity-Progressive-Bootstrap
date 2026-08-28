'use strict';

const crypto = require('crypto');

class TrajectoryEventSource {
  constructor(languageServer, options = {}) {
    this._ls = languageServer;
    this._now = options.now || (() => Date.now());
    this._idleFinalMs = Math.max(500, Number(options.idleFinalMs) || 2500);
    this._maxEvents = Math.max(100, Number(options.maxEvents) || 2000);
    this._states = new Map();
  }

  get ready() { return Boolean(this._ls && this._ls.ready); }

  async listEvents(conversationId, after = 0) {
    if (!this.ready) throw new Error('agent event source is unavailable');
    const trajectory = await this._ls.getTrajectory(conversationId);
    const state = this._state(conversationId);
    observeTrajectory(state, conversationId, trajectory, this._now(), this._idleFinalMs);
    if (state.events.length > this._maxEvents) state.events.splice(0, state.events.length - this._maxEvents);
    return state.events.filter(event => event.seq > after);
  }

  _state(conversationId) {
    let state = this._states.get(conversationId);
    if (!state) {
      state = { nextSeq: 1, steps: new Map(), events: [] };
      this._states.set(conversationId, state);
    }
    return state;
  }
}

function observeTrajectory(state, conversationId, raw, nowMs, idleFinalMs) {
  const steps = raw && raw.trajectory && Array.isArray(raw.trajectory.steps) ? raw.trajectory.steps : [];
  steps.forEach((step, index) => {
    const text = plannerText(step);
    if (!text) return;
    const streamKey = `${conversationId}:step:${index}`;
    let snapshot = state.steps.get(index);
    if (!snapshot) {
      snapshot = { text: '', changedAt: nowMs, finalizedText: '' };
      state.steps.set(index, snapshot);
    }
    if (text !== snapshot.text) {
      snapshot.text = text;
      snapshot.changedAt = nowMs;
      emit(state, conversationId, index, 'agent_delta', streamKey, text, nowMs, false);
      return;
    }
    if (snapshot.finalizedText !== text && nowMs - snapshot.changedAt >= idleFinalMs) {
      snapshot.finalizedText = text;
      emit(state, conversationId, index, 'agent_message', streamKey, text, nowMs, true);
    }
  });
}

function plannerText(step) {
  const response = step && step.plannerResponse;
  if (!response || typeof response !== 'object') return '';
  const text = typeof response.response === 'string' && response.response.trim()
    ? response.response
    : (typeof response.modifiedResponse === 'string' ? response.modifiedResponse : '');
  return text.trim();
}

function emit(state, conversationId, stepIndex, type, streamKey, text, nowMs, final) {
  const sourceEventId = stableEventId(conversationId, stepIndex, type, text);
  if (state.events.some(event => event.sourceEventId === sourceEventId)) return;
  state.events.push({
    seq: state.nextSeq++,
    type,
    sourceEventId,
    streamKey,
    timestamp: new Date(nowMs).toISOString(),
    payload: { conversationId, stepIndex, text, final }
  });
}

function stableEventId(conversationId, stepIndex, type, text) {
  return crypto.createHash('sha256').update(String(conversationId)).update('\0').update(String(stepIndex)).update('\0').update(type).update('\0').update(text).digest('hex');
}

module.exports = { TrajectoryEventSource, observeTrajectory, plannerText, stableEventId };
