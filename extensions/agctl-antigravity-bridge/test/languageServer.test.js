'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { extractArg, parseNetstatPorts, workspaceProcessHint, validateConversationId } = require('../src/languageServer');

test('extractArg accepts equals and spaced CLI syntax', () => {
  assert.equal(extractArg('x --csrf_token=abc --port 2', 'csrf_token'), 'abc');
  assert.equal(extractArg('x --csrf_token abc --port 2', 'csrf_token'), 'abc');
  assert.equal(extractArg('x --other abc', 'csrf_token'), null);
});

test('netstat parser keeps only loopback listeners owned by PID', () => {
  const output = [
    'TCP    127.0.0.1:51000    0.0.0.0:0    LISTENING    123',
    'TCP    0.0.0.0:52000      0.0.0.0:0    LISTENING    123',
    'TCP    127.0.0.1:53000    0.0.0.0:0    LISTENING    999'
  ].join('\n');
  assert.deepEqual(parseNetstatPorts(output, 123), [51000]);
});

test('workspace hint is deterministic and conversation id is bounded', () => {
  assert.equal(workspaceProcessHint('D:\\Work\\My-Repo'), 'work_my_repo');
  assert.equal(validateConversationId('abc-123'), 'abc-123');
  assert.throws(() => validateConversationId('../bad'));
});
