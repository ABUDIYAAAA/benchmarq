'use strict';

const { app } = require('electron');
const { EventEmitter } = require('node:events');
const { NO_LOCKDOWN } = require('./config');

// Shortcuts swallowed in the exam window: reload, close/quit, devtools,
// minimize/hide and fullscreen toggles. "Mod" is Cmd on macOS, Ctrl elsewhere.
const BLOCKED_KEYS = new Set([
  'Mod+R', 'Mod+Shift+R', 'F5',
  'Mod+W', 'Mod+Q', 'Alt+F4',
  'F12', 'Mod+Shift+I', 'Mod+Alt+I', 'Mod+Shift+J', 'Mod+Alt+J', 'Mod+Shift+C', 'Mod+Alt+C',
  'Mod+M', 'Mod+H', 'Mod+Alt+H',
  'F11', 'Mod+Ctrl+F',
]);

function comboOf(input) {
  const mod = process.platform === 'darwin' ? input.meta : input.control;
  const parts = [];
  if (mod) parts.push('Mod');
  if (process.platform === 'darwin' && input.control) parts.push('Ctrl');
  if (input.shift) parts.push('Shift');
  if (input.alt) parts.push('Alt');
  parts.push(input.key.length === 1 ? input.key.toUpperCase() : input.key);
  return parts.join('+');
}

/**
 * Keeps the exam window full screen and in front. While an attempt runs,
 * losing focus or leaving full screen is undone immediately and reported
 * through the 'violation' event (tab_switch / fullscreen_exit).
 */
class Lockdown extends EventEmitter {
  constructor(win) {
    super();
    this.win = win;
    this.active = false;
    this.blurredAt = null;

    win.webContents.on('before-input-event', (event, input) => {
      if (input.type === 'keyDown' && BLOCKED_KEYS.has(comboOf(input))) event.preventDefault();
    });

    win.on('blur', () => {
      if (!this.active) return;
      this.blurredAt = Date.now();
      setImmediate(() => this.refocus());
    });
    win.on('focus', () => {
      if (!this.active || !this.blurredAt) return;
      const duration = Date.now() - this.blurredAt;
      this.blurredAt = null;
      this.emit('violation', { type: 'tab_switch', details: { duration_ms: duration } });
    });
    win.on('leave-full-screen', () => {
      if (!this.active) return;
      this.emit('violation', { type: 'fullscreen_exit', details: {} });
      setImmediate(() => this.enforce());
    });
    win.on('minimize', () => {
      if (this.active) setImmediate(() => this.enforce());
    });
  }

  /** Kiosk mode; on macOS it also hides the Dock and disables app switching. */
  enforce() {
    const win = this.win;
    if (win.isDestroyed() || NO_LOCKDOWN) return;
    if (win.isMinimized()) win.restore();
    win.setKiosk(true);
    win.setAlwaysOnTop(true, 'screen-saver');
    win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true });
    win.focus();
  }

  refocus() {
    if (this.win.isDestroyed() || NO_LOCKDOWN) return;
    this.win.show();
    this.win.focus();
    app.focus({ steal: true });
  }

  activate() {
    this.active = true;
    this.enforce();
  }

  deactivate() {
    this.active = false;
    if (!this.win.isDestroyed()) {
      this.win.setAlwaysOnTop(false);
      this.win.setKiosk(false);
    }
  }
}

module.exports = { Lockdown, comboOf };
