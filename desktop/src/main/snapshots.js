'use strict';

const { BrowserWindow, desktopCapturer, ipcMain, screen, systemPreferences } = require('electron');
const path = require('node:path');
const { allowCamera } = require('./hardening');
const { SNAPSHOT_INTERVAL_MS, SNAPSHOT_JPEG_QUALITY } = require('./config');

const FRAME_TIMEOUT_MS = 8000;

/** Resolves null if promise has not settled within ms. */
function withTimeout(promise, ms) {
  let timer;
  return Promise.race([promise, new Promise((resolve) => (timer = setTimeout(resolve, ms, null)))]).finally(() => clearTimeout(timer));
}

/**
 * Captures a webcam frame and the screen every 30 seconds while an attempt
 * runs and uploads both to the server.
 *
 * The camera lives in a hidden local window that only this module talks to,
 * so the exam page cannot stop or replace the capture. On macOS, screen
 * capture shows real content only once Benchmarq Exam has Screen Recording
 * permission; otherwise the image contains just the desktop wallpaper.
 */
class SnapshotCapture {
  constructor(runtime) {
    this.runtime = runtime;
    this.timer = null;
    this.cameraWin = null;
    this.pending = new Map(); // request id -> { resolve, timer }
    this.nextId = 1;

    this.onFrame = (event, { id, buffer, error } = {}) => {
      if (!this.cameraWin || event.sender !== this.cameraWin.webContents) return;
      const req = this.pending.get(id);
      if (!req) return;
      this.pending.delete(id);
      clearTimeout(req.timer);
      req.resolve(buffer && !error ? Buffer.from(buffer) : null);
    };
  }

  async start() {
    if (this.timer) return;
    if (process.platform === 'darwin') {
      await systemPreferences.askForMediaAccess('camera').catch(() => false);
    }
    this.#openCamera();
    ipcMain.on('capture:frame', this.onFrame);
    // First snapshot shortly after the attempt starts, then every interval.
    setTimeout(() => this.#tick(), 5000);
    this.timer = setInterval(() => this.#tick(), SNAPSHOT_INTERVAL_MS);
  }

  stop() {
    clearInterval(this.timer);
    this.timer = null;
    ipcMain.removeListener('capture:frame', this.onFrame);
    for (const req of this.pending.values()) {
      clearTimeout(req.timer);
      req.resolve(null);
    }
    this.pending.clear();
    if (this.cameraWin && !this.cameraWin.isDestroyed()) this.cameraWin.destroy();
    this.cameraWin = null;
  }

  #openCamera() {
    this.cameraWin = new BrowserWindow({
      show: false,
      width: 640,
      height: 480,
      webPreferences: {
        preload: path.join(__dirname, '..', 'preload', 'capture.js'),
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
        backgroundThrottling: false,
      },
    });
    allowCamera(this.cameraWin.webContents);
    this.cameraWin.loadFile(path.join(__dirname, '..', 'capture', 'camera.html'));
  }

  async #tick() {
    if (!this.timer || !this.runtime.attemptRunning()) return;
    const [webcam, screenShot] = await Promise.all([this.#webcam(), this.#screen()]);
    if (!webcam) console.warn('snapshots: no webcam frame');
    if (!screenShot) console.warn('snapshots: no screen capture');
    if (!webcam && !screenShot) return;
    await this.runtime.uploadSnapshots({ webcam, screen: screenShot });
  }

  /** Asks the camera window for a frame; resolves null on failure. */
  #webcam() {
    if (!this.cameraWin || this.cameraWin.isDestroyed()) this.#openCamera();
    const id = this.nextId++;
    return new Promise((resolve) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        resolve(null);
      }, FRAME_TIMEOUT_MS);
      this.pending.set(id, { resolve, timer });
      this.cameraWin.webContents.send('capture:take', id);
    });
  }

  /**
   * Captures the primary display; resolves null on failure. On macOS,
   * getSources can wait indefinitely on the Screen Recording prompt, so it
   * is bounded like the webcam frame.
   */
  async #screen() {
    try {
      const display = screen.getPrimaryDisplay();
      const sources = await withTimeout(
        desktopCapturer.getSources({
          types: ['screen'],
          thumbnailSize: { width: display.size.width, height: display.size.height },
        }),
        FRAME_TIMEOUT_MS,
      );
      if (!sources) return null;
      const source = sources.find((s) => s.display_id === String(display.id)) ?? sources[0];
      if (!source || source.thumbnail.isEmpty()) return null;
      return source.thumbnail.toJPEG(SNAPSHOT_JPEG_QUALITY);
    } catch {
      return null;
    }
  }
}

module.exports = { SnapshotCapture };
