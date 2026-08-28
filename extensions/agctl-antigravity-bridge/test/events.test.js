'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { TrajectoryEventSource, plannerText, stableEventId } = require('../src/events');

class FakeLS {
  constructor(raw) { this.ready = true; this.raw = raw; }
  async getTrajectory() { return this.raw; }
}

function trajectory(text, modified = '') {
  return { trajectory: { steps: [{ plannerResponse: { response: text, modifiedResponse: modified } }] } };
}

test('plannerText prefers response then modifiedResponse', () => {
  assert.equal(plannerText({ plannerResponse: { response: 'primary', modifiedResponse: 'fallback' } }), 'primary');
  assert.equal(plannerText({ plannerResponse: { response: '', modifiedResponse: 'fallback' } }), 'fallback');
  assert.equal(plannerText({ other: true }), '');
});

test('trajectory source emits deterministic delta then final for one stream', async () => {
  let now = 1000;
  const source = new TrajectoryEventSource(new FakeLS(trajectory('hello')), { now: () => now, idleFinalMs: 500 });
  let events = await source.listEvents('conversation-1', 0);
  assert.equal(events.length, 1);
  assert.equal(events[0].type, 'agent_delta');
  assert.equal(events[0].streamKey, 'conversation-1:step:0');
  assert.equal(events[0].payload.text, 'hello');
  now += 600;
  events = await source.listEvents('conversation-1', 1);
  assert.equal(events.length, 1);
  assert.equal(events[0].type, 'agent_message');
  assert.equal(events[0].payload.final, true);
});

test('changed planner response stays in same stream and creates no source-id duplicate', async () => {
  let now = 1000;
  const ls = new FakeLS(trajectory('a'));
  const source = new TrajectoryEventSource(ls, { now: () => now, idleFinalMs: 500 });
  await source.listEvents('c', 0);
  ls.raw = trajectory('ab');
  now += 100;
  const events = await source.listEvents('c', 1);
  assert.equal(events.length, 1);
  assert.equal(events[0].type, 'agent_delta');
  assert.equal(events[0].streamKey, 'c:step:0');
  assert.notEqual(events[0].sourceEventId, stableEventId('c', 0, 'agent_delta', 'a'));
  const replay = await source.listEvents('c', 2);
  assert.equal(replay.length, 0);
});

test('unknown trajectory steps are not misclassified as messages', async () => {
  const source = new TrajectoryEventSource(new FakeLS({ trajectory: { steps: [{ toolCall: { name: 'x' } }] } }), { now: () => 1000 });
  assert.deepEqual(await source.listEvents('c', 0), []);
});
