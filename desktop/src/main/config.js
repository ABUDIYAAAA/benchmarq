'use strict';

const { app } = require('electron');

// Packaged builds ignore environment overrides: otherwise anyone could point
// the client at a page of their own and hand it the exam bridge.
const overridable = !app.isPackaged;

function setting(envName, packagedValue, devValue) {
  if (overridable && process.env[envName]) return process.env[envName];
  return app.isPackaged ? packagedValue : devValue;
}

const APP_URL = new URL(setting('BENCHMARQ_APP_URL', 'https://app.benchmarq.io', 'http://localhost:3000'));
const API_URL = setting('BENCHMARQ_API_URL', 'https://api.benchmarq.io/api/v1', 'http://localhost:8000/api/v1').replace(/\/+$/, '');

const PROTOCOL = 'benchmarq';

/**
 * Applications the candidate may keep running during the exam, matched
 * case-insensitively against the process or app bundle name. Everything else
 * with a window must be closed before the exam can start.
 */
const ALLOWED_APPS = {
  darwin: ['Google Chrome'],
  win32: ['chrome.exe'],
  linux: ['chrome', 'google-chrome'],
};

/**
 * Operating-system processes that own windows but are part of the desktop
 * itself. They are never reported or terminated.
 */
const SYSTEM_APPS = {
  darwin: ['Finder', 'Dock', 'SystemUIServer', 'ControlCenter', 'NotificationCenter', 'loginwindow', 'WindowManager', 'Spotlight', 'TextInputMenuAgent'],
  win32: [
    'explorer.exe', 'ApplicationFrameHost.exe', 'ShellExperienceHost.exe', 'StartMenuExperienceHost.exe',
    'SearchHost.exe', 'SearchApp.exe', 'TextInputHost.exe', 'SystemSettings.exe', 'LockApp.exe', 'dwm.exe',
    'csrss.exe', 'winlogon.exe', 'ctfmon.exe', 'sihost.exe', 'taskhostw.exe', 'RuntimeBroker.exe',
  ],
  linux: ['gnome-shell', 'plasmashell', 'kwin_x11', 'kwin_wayland', 'Xorg', 'Xwayland', 'xfwm4', 'xfce4-panel'],
};

/**
 * Remote-access, screen-sharing, virtual-camera and AI "interview assistant"
 * tools. These are blocked even when they run without a window. Matched as
 * lowercase substrings of the process name.
 */
const BLOCKED_PROCESSES = [
  'teamviewer', 'anydesk', 'rustdesk', 'parsec', 'remoting_host', 'screenconnect', 'connectwise',
  'vncserver', 'vncviewer', 'tightvnc', 'ultravnc', 'realvnc', 'splashtop', 'ammyy', 'ultraviewer', 'logmein',
  'obs', 'streamlabs', 'xsplit', 'manycam', 'snap camera', 'camtwist', 'loom', 'screenflow', 'bandicam',
  'zoom.us', 'zoom', 'discord', 'skype', 'msteams', 'teams', 'slack', 'webex', 'whatsapp', 'telegram',
  'cluely', 'interviewcoder', 'interview coder', 'finalround', 'lockedin',
];

// Development only: skip kiosk mode so the UI can be worked on.
const NO_LOCKDOWN = overridable && process.env.BENCHMARQ_NO_LOCKDOWN === '1';

module.exports = {
  APP_URL,
  APP_ORIGIN: APP_URL.origin,
  EXAM_URL: new URL('/exam', APP_URL).toString(),
  API_URL,
  PROTOCOL,
  ALLOWED_APPS: ALLOWED_APPS[process.platform] ?? [],
  SYSTEM_APPS: SYSTEM_APPS[process.platform] ?? [],
  BLOCKED_PROCESSES,
  NO_LOCKDOWN,
  APP_SCAN_INTERVAL_MS: 15_000,
  REQUEST_TIMEOUT_MS: 15_000,
};
