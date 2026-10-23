'use strict';

const { app, safeStorage } = require('electron');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

// Small secrets (device ID, session token) are encrypted with the OS keychain
// (Keychain / DPAPI / libsecret) through safeStorage and written atomically
// with owner-only permissions.

function filePath(name) {
  return path.join(app.getPath('userData'), `${name}.bin`);
}

function canEncrypt() {
  return safeStorage.isEncryptionAvailable();
}

function write(name, value) {
  if (!canEncrypt()) throw new Error('Secure storage is unavailable on this system');
  const target = filePath(name);
  const tmp = `${target}.${crypto.randomBytes(4).toString('hex')}.tmp`;
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(tmp, safeStorage.encryptString(JSON.stringify(value)), { mode: 0o600 });
  fs.renameSync(tmp, target);
}

function read(name) {
  try {
    if (!canEncrypt()) return null;
    return JSON.parse(safeStorage.decryptString(fs.readFileSync(filePath(name))));
  } catch {
    return null; // missing, corrupted or encrypted by another user/machine
  }
}

function remove(name) {
  fs.rmSync(filePath(name), { force: true });
}

module.exports = { read, write, remove, canEncrypt };
