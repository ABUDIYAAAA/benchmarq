'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');

// Stub the electron module so main-process logic can be tested under plain Node.
const realLoad = Module._load;
Module._load = function (request, ...rest) {
  if (request === 'electron') {
    return {
      app: { isPackaged: false, getPath: () => '/tmp', commandLine: { hasSwitch: () => false } },
      safeStorage: { isEncryptionAvailable: () => false },
      clipboard: {}, screen: { on() {}, off() {} }, powerMonitor: { on() {}, off() {} }, globalShortcut: {},
    };
  }
  return realLoad.call(this, request, ...rest);
};

const { findOffenders, isBlocked } = require('../src/main/integrity/processes');
const { comboOf } = require('../src/main/lockdown');
const { sanitizeResponse } = require('../src/main/runtime');

const ID = '0b9a2f6c-1f4e-4d7a-9c3b-2e5f6a7b8c9d';

test('blocklist matches whole words only', () => {
  assert.ok(isBlocked('OBS Studio'));
  assert.ok(isBlocked('TeamViewer.exe'));
  assert.ok(isBlocked('zoom.us'));
  assert.ok(isBlocked('Cluely'));
  assert.ok(!isBlocked('Obsidian'));
  assert.ok(!isBlocked('Zoomit-like-editor-zoomer'));
});

test('offenders: only blocklisted tools, grouped per app', () => {
  const system = process.platform === 'win32' ? 'explorer.exe' : process.platform === 'darwin' ? 'Finder' : 'gnome-shell';
  const procs = [
    { pid: 1, name: 'Google Chrome' },
    { pid: 2, name: system },
    { pid: 3, name: 'Notes' },
    { pid: 4, name: 'XProtect' },
    { pid: 5, name: 'AnyDesk' },
    { pid: 6, name: 'Discord' },
    { pid: 7, name: 'Discord' },
    { pid: 8, name: 'zoom.us' },
  ];
  const offenders = findOffenders(procs, new Set([8]));
  assert.deepEqual(
    offenders.map((o) => [o.name, o.reason, o.pids]),
    [['AnyDesk', 'blocked', [5]], ['Discord', 'blocked', [6, 7]]],
  );
});

test('shortcut normalization', () => {
  const mod = process.platform === 'darwin' ? { meta: true } : { control: true };
  assert.equal(comboOf({ ...mod, key: 'r' }), 'Mod+R');
  assert.equal(comboOf({ ...mod, shift: true, key: 'I' }), 'Mod+Shift+I');
  assert.equal(comboOf({ key: 'F12' }), 'F12');
});

test('response sanitization rejects anything unexpected', () => {
  assert.deepEqual(sanitizeResponse({ question_id: ID, text: 'hi', evil: 1 }), { question_id: ID, text: 'hi' });
  assert.deepEqual(sanitizeResponse({ question_id: ID, selected_option_ids: [ID, ID] }), { question_id: ID, selected_option_ids: [ID] });
  assert.deepEqual(sanitizeResponse({ question_id: ID, flagged: 1, clear: 'yes' }), { question_id: ID, flagged: true });
  assert.throws(() => sanitizeResponse({ question_id: '../../admin' }));
  assert.throws(() => sanitizeResponse({ question_id: ID, selected_option_id: 'x' }));
  assert.throws(() => sanitizeResponse({ question_id: ID, text: 'x'.repeat(500_001) }));
});
