'use strict';

const { app, BrowserWindow, dialog } = require('electron');
const path = require('node:path');
const { EXAM_URL, PROTOCOL } = require('./config');
const { harden } = require('./hardening');
const { ExamController } = require('./controller');

app.enableSandbox();

if (!app.requestSingleInstanceLock()) {
  // Another instance owns the exam; it receives our argv (and deep link).
  app.exit(0);
}

// ---------------------------------------------------------------------------
// Deep links: benchmarq://launch?code=XXXX
// ---------------------------------------------------------------------------

if (process.defaultApp && process.argv.length >= 2) {
  app.setAsDefaultProtocolClient(PROTOCOL, process.execPath, [path.resolve(process.argv[1])]);
} else {
  app.setAsDefaultProtocolClient(PROTOCOL);
}

function launchCodeFrom(argv) {
  for (const arg of argv) {
    if (typeof arg !== 'string' || !arg.startsWith(`${PROTOCOL}://`)) continue;
    try {
      const url = new URL(arg);
      if (url.hostname === 'launch') return url.searchParams.get('code');
    } catch {
      // ignore malformed links
    }
  }
  return null;
}

let pendingCode = launchCodeFrom(process.argv);
let controller = null;
let win = null;

app.on('open-url', (event, url) => {
  // macOS delivers deep links here, possibly before 'ready'.
  event.preventDefault();
  const code = launchCodeFrom([url]);
  if (!code) return;
  if (controller) switchSession(code);
  else pendingCode = code;
});

app.on('second-instance', (_event, argv) => {
  const code = launchCodeFrom(argv);
  if (code && controller) switchSession(code);
  if (win) {
    if (win.isMinimized()) win.restore();
    win.focus();
  }
});

/** A new launch code replaces the current session unless an attempt is running. */
async function switchSession(code) {
  if (controller.runtime.attemptRunning()) {
    dialog.showMessageBox(win, { type: 'info', message: 'An exam is already in progress in this window.' });
    return;
  }
  await controller.begin(code);
  win.loadURL(EXAM_URL);
}

// ---------------------------------------------------------------------------
// Window
// ---------------------------------------------------------------------------

async function createWindow() {
  win = new BrowserWindow({
    width: 1280,
    height: 820,
    show: false,
    backgroundColor: '#ffffff',
    title: 'Benchmarq Exam',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, '..', 'preload', 'index.js'),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      webSecurity: true,
      webviewTag: false,
      spellcheck: false,
      devTools: !app.isPackaged,
      backgroundThrottling: false, // keep timers and heartbeats running
    },
  });
  win.setMenu(null);

  controller = new ExamController(win);
  controller.begin(pendingCode);
  pendingCode = null;

  win.on('close', (event) => {
    if (controller.runtime.attemptRunning()) {
      event.preventDefault();
      controller.requestQuit();
    }
  });

  win.webContents.on('did-fail-load', async (_e, code, description, _url, isMainFrame) => {
    if (!isMainFrame || code === -3) return; // -3: aborted by a navigation
    const { response } = await dialog.showMessageBox(win, {
      type: 'error',
      buttons: ['Retry', 'Quit'],
      message: 'Could not load the exam',
      detail: `${description}. Check your internet connection and try again.`,
    });
    if (response === 0) win.loadURL(EXAM_URL);
    else app.exit(0);
  });

  // Debugging or a VM quits before anything loads. Everything else (no
  // launch link, apps to close) is explained by the page itself; kiosk mode
  // starts once the system check passes.
  await controller.preflight();

  win.loadURL(EXAM_URL);
  win.once('ready-to-show', () => win.show());
}

app.whenReady().then(() => {
  harden();
  createWindow();
});

app.on('before-quit', (event) => {
  if (controller?.runtime.attemptRunning()) {
    event.preventDefault();
    controller.requestQuit();
  }
});

app.on('window-all-closed', () => {
  controller?.dispose();
  app.quit();
});
