'use strict';

// Runs sandboxed in an isolated world and exposes a small, frozen API to the
// exam page: no ipcRenderer, no Node. The main process re-validates the
// sender and every argument.

const { contextBridge, ipcRenderer } = require('electron');

const EVENTS = new Set([
  'runtime:state',
  'runtime:attempt',
  'runtime:connection',
  'runtime:ended',
  'runtime:invalid',
  'runtime:violation',
]);

// Errors cross the context bridge as plain objects: Error instances lose
// every property except the message on the way.
async function call(channel, ...args) {
  const result = await ipcRenderer.invoke(channel, ...args);
  if (result?.ok) return result.data;
  const { status = 500, message = 'Unexpected error', fields = null } = result?.error ?? {};
  throw { name: 'BridgeError', status, message, fields };
}

function on(channel, callback) {
  if (!EVENTS.has(channel) || typeof callback !== 'function') throw new Error(`Unknown event: ${channel}`);
  const listener = (_event, payload) => callback(payload);
  ipcRenderer.on(channel, listener);
  return () => ipcRenderer.removeListener(channel, listener);
}

const api = Object.freeze({
  session: Object.freeze({
    bootstrap: () => call('session:bootstrap'),
  }),
  system: Object.freeze({
    check: () => call('system:check'),
    closeApps: () => call('system:closeApps'),
    requestCamera: () => call('system:requestCamera'),
  }),
  runtime: Object.freeze({
    refresh: () => call('runtime:refresh'),
    start: () => call('runtime:start'),
    resumeAttempt: () => call('runtime:resumeAttempt'),
    enterSection: (id) => call('runtime:enterSection', String(id)),
    finishSection: (id) => call('runtime:finishSection', String(id)),
    question: () => call('runtime:question'),
    navigate: (position) => call('runtime:navigate', Number(position)),
    saveResponse: (input) => call('runtime:saveResponse', structuredClone(input)),
    pause: () => call('runtime:pause'),
    resume: () => call('runtime:resume'),
    submit: () => call('runtime:submit'),
  }),
  app: Object.freeze({
    quit: () => call('app:quit'),
    openPortal: () => call('app:openPortal'),
    enterFullscreen: () => call('app:enterFullscreen'),
  }),
  on,
});

if (window.top === window) {
  contextBridge.exposeInMainWorld('benchmarq', api);
}
