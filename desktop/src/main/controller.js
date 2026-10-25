'use strict';

const { app, dialog, ipcMain, shell, systemPreferences } = require('electron');
const { APP_ORIGIN, APP_SCAN_INTERVAL_MS, PORTAL_URL } = require('./config');
const { Runtime, RuntimeError } = require('./runtime');
const { Lockdown } = require('./lockdown');
const checks = require('./integrity/checks');
const { SnapshotCapture } = require('./snapshots');
const { allowCamera } = require('./hardening');

// Development only: skip the pre-exam gate so the app can run next to an
// editor and terminal. Packaged builds never honour it.
const RELAXED = !app.isPackaged && process.env.BENCHMARQ_RELAXED === '1';

/**
 * Owns the exam window: the server session, the system checks shown before
 * an attempt can start, the lockdown during an attempt, and the IPC surface
 * exposed through the preload bridge.
 */
class ExamController {
  constructor(win) {
    this.win = win;
    this.runtime = new Runtime();
    this.lockdown = new Lockdown(win);
    this.snapshots = new SnapshotCapture(this.runtime);
    this.scanTimer = null;
    this.ready = Promise.resolve();
    this.cleared = false;
    this.bootError = null;
    this.channels = [];

    // The waiting room shows a camera preview before the exam starts.
    allowCamera(win.webContents);
    this.#wire();
    this.#registerIpc();
  }

  /** Opens a session from a launch code, or resumes the stored one. */
  begin(launchCode) {
    this.ready = (async () => {
      this.bootError = null;
      try {
        if (launchCode) await this.runtime.openSession(launchCode);
        else if (this.runtime.hasSession()) await this.runtime.refresh();
      } catch (err) {
        this.bootError = err instanceof RuntimeError ? err.toJSON() : { status: 0, message: String(err?.message ?? err) };
      }
    })();
    return this.ready;
  }

  // ----- Gate ------------------------------------------------------------------

  /**
   * Runs before the window loads anything. Tampering and virtual machines
   * cannot be fixed by the candidate, so they quit with a native dialog. Open
   * applications are checked later, in the page (see #checkSystem).
   */
  async preflight() {
    if (RELAXED) return;

    if (checks.tampered()) {
      await this.#fatal('Benchmarq Exam was started with debugging or modified settings. Start it normally from the portal.');
    }
    if (await checks.virtualMachine()) {
      await this.#fatal('Benchmarq Exam cannot run inside a virtual machine. Use a physical computer.');
    }
  }

  /** Lists the blocked applications that must be closed before the exam can start. */
  async #checkSystem() {
    const apps = RELAXED ? [] : await checks.disallowedApps();
    this.cleared = apps.length === 0;
    return { clear: this.cleared, apps: apps.map((a) => a.name) };
  }

  /** Asks macOS for camera access; other platforms grant it per request. */
  async #requestCamera() {
    if (process.platform !== 'darwin') return true;
    return systemPreferences.askForMediaAccess('camera').catch(() => false);
  }

  /** Closes the reported apps and restarts the app; returns what survived. */
  async #closeApps() {
    const apps = RELAXED ? [] : await checks.disallowedApps();
    const remaining = apps.length ? await checks.closeApps(apps) : [];
    if (remaining.length === 0) {
      app.relaunch();
      await this.#exit(0);
    }
    this.cleared = false;
    return { clear: false, apps: remaining };
  }

  /** Starting or resuming an attempt always re-checks the machine. */
  async #requireClear() {
    const { clear } = await this.#checkSystem();
    if (!clear) throw new RuntimeError(412, 'Close the blocked applications before starting the exam.');
  }

  async #fatal(message) {
    await dialog.showMessageBox({ type: 'error', title: 'Benchmarq Exam', message });
    await this.#exit(1);
  }

  /** Exits and never resolves, so callers stop where they are. */
  #exit(code) {
    app.exit(code);
    return new Promise(() => {});
  }

  // ----- Attempt monitoring ------------------------------------------------------

  #startMonitoring() {
    this.lockdown.activate();
    this.snapshots.start().catch(() => {});
    if (this.scanTimer || RELAXED) return;
    // Applications opened during the exam are closed without asking: the
    // candidate already agreed to this at the gate.
    this.scanTimer = setInterval(async () => {
      const apps = await checks.disallowedApps().catch(() => []);
      if (apps.length) await checks.closeApps(apps).catch(() => {});
    }, APP_SCAN_INTERVAL_MS);
  }

  #stopMonitoring() {
    this.snapshots.stop();
    clearInterval(this.scanTimer);
    this.scanTimer = null;
    this.lockdown.deactivate();
  }

  #wire() {
    const rt = this.runtime;
    rt.on('state', (s) => this.#send('runtime:state', s));
    rt.on('attempt', (a) => this.#send('runtime:attempt', a));
    rt.on('connection', (c) => this.#send('runtime:connection', c));
    rt.on('ended', (a) => {
      this.#stopMonitoring();
      this.#send('runtime:ended', a);
    });
    rt.on('invalid', (err) => {
      this.#stopMonitoring();
      this.#send('runtime:invalid', err);
    });
    this.lockdown.on('violation', async ({ type, details }) => {
      const result = await rt.reportEvent(type, details);
      if (result) this.#send('runtime:violation', { type, ...result });
    });
  }

  // ----- IPC ---------------------------------------------------------------------

  #send(channel, payload) {
    if (!this.win.isDestroyed()) this.win.webContents.send(channel, payload);
  }

  /** Accepts IPC only from the exam window's top frame on the exam origin. */
  #trusted(event) {
    const frame = event.senderFrame;
    if (!frame || event.sender !== this.win.webContents || frame !== this.win.webContents.mainFrame) return false;
    try {
      return new URL(frame.url).origin === APP_ORIGIN;
    } catch {
      return false;
    }
  }

  #handle(channel, fn) {
    this.channels.push(channel);
    ipcMain.handle(channel, async (event, ...args) => {
      if (!this.#trusted(event)) return { ok: false, error: { status: 403, message: 'Forbidden' } };
      try {
        return { ok: true, data: await fn(...args) };
      } catch (err) {
        const error = err instanceof RuntimeError ? err.toJSON() : { status: 500, message: err?.message || 'Unexpected error' };
        return { ok: false, error };
      }
    });
  }

  #registerIpc() {
    const rt = this.runtime;
    this.#handle('session:bootstrap', async () => {
      await this.ready;
      // The system check only matters once there is an exam to open.
      const system = rt.state ? await this.#checkSystem() : null;
      return { status: rt.state ? 'active' : 'none', error: this.bootError, state: rt.state, system };
    });
    this.#handle('system:check', () => this.#checkSystem());
    this.#handle('system:closeApps', () => this.#closeApps());
    this.#handle('runtime:refresh', () => rt.refresh());
    this.#handle('runtime:start', async () => {
      await this.#requireClear();
      const state = await rt.start();
      this.#startMonitoring();
      return state;
    });
    this.#handle('runtime:resumeAttempt', async () => {
      await this.#requireClear();
      const state = await rt.refresh();
      if (!rt.attemptRunning()) throw new RuntimeError(409, 'There is no attempt in progress');
      this.#startMonitoring();
      return state;
    });
    this.#handle('runtime:enterSection', (id) => rt.enterSection(id));
    this.#handle('runtime:finishSection', (id) => rt.finishSection(id));
    this.#handle('runtime:question', () => rt.currentQuestion());
    this.#handle('runtime:navigate', (position) => rt.navigate(position));
    this.#handle('runtime:saveResponse', (input) => rt.saveResponse(input));
    this.#handle('runtime:pause', () => rt.pause());
    this.#handle('runtime:resume', () => rt.resume());
    this.#handle('runtime:submit', () => rt.submit());
    this.#handle('app:quit', () => this.requestQuit());
    this.#handle('app:openPortal', () => shell.openExternal(PORTAL_URL));
    this.#handle('system:requestCamera', () => this.#requestCamera());
    // Kiosk mode starts when the candidate asks for it in the waiting room,
    // and in any case when the attempt starts (#startMonitoring).
    this.#handle('app:enterFullscreen', () => {
      this.lockdown.enforce();
      return true;
    });
  }

  /** Quitting is free outside an attempt; during one it needs confirmation. */
  async requestQuit() {
    if (this.runtime.attemptRunning()) {
      const { response } = await dialog.showMessageBox(this.win, {
        type: 'warning',
        buttons: ['Keep working', 'Leave exam'],
        defaultId: 0,
        cancelId: 0,
        message: 'Your exam is still running.',
        detail: 'Leaving does not submit or pause the exam; the timer keeps running. You can return by launching the exam again from the portal.',
      });
      if (response !== 1) return false;
    }
    this.#stopMonitoring();
    app.exit(0);
    return true;
  }

  dispose() {
    this.#stopMonitoring();
    this.runtime.stopHeartbeat();
    for (const channel of this.channels) ipcMain.removeHandler(channel);
  }
}

module.exports = { ExamController };
