'use strict';

const crypto = require('node:crypto');
const store = require('./store');

// The device ID binds an exam session to this installation. The server
// rejects session tokens presented with any other device ID, so a leaked
// token is useless elsewhere.

let cached = null;

function deviceId() {
  if (cached) return cached;
  const saved = store.read('device');
  if (saved && typeof saved.id === 'string' && /^[a-f0-9]{64}$/.test(saved.id)) {
    cached = saved.id;
    return cached;
  }
  cached = crypto.randomBytes(32).toString('hex');
  store.write('device', { id: cached, createdAt: new Date().toISOString() });
  return cached;
}

module.exports = { deviceId };
