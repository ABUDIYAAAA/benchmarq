"use strict";

const { app } = require("electron");

// Packaged builds ignore environment overrides: otherwise anyone could point
// the client at a page of their own and hand it the exam bridge. Their URLs
// come from app-config.json, written at build time (scripts/write-app-config.js)
// and sealed inside the integrity-checked app.asar.
const overridable = !app.isPackaged;

let built = {};
try {
  built = require("./app-config.json");
} catch {
  // not a configured build
}

function setting(envName, key, packagedValue, devValue = packagedValue) {
  if (overridable && process.env[envName]) return process.env[envName];
  if (app.isPackaged) return built[key] || packagedValue;
  return devValue;
}

const APP_URL = new URL(
  setting("BENCHMARQ_APP_URL", "appUrl", "http://localhost:3000"),
);
const API_URL = setting(
  "BENCHMARQ_API_URL",
  "apiUrl",
  "http://localhost:8000/api/v1",
).replace(/\/+$/, "");

const PROTOCOL = "benchmarq";

/**
 * Operating-system processes that own windows but are part of the desktop
 * itself. They are never reported or terminated.
 */
const SYSTEM_APPS = {
  darwin: [
    "Finder",
    "Dock",
    "SystemUIServer",
    "ControlCenter",
    "NotificationCenter",
    "loginwindow",
    "WindowManager",
    "Spotlight",
    "TextInputMenuAgent",
  ],
  win32: [
    "explorer.exe",
    "ApplicationFrameHost.exe",
    "ShellExperienceHost.exe",
    "StartMenuExperienceHost.exe",
    "SearchHost.exe",
    "SearchApp.exe",
    "TextInputHost.exe",
    "SystemSettings.exe",
    "LockApp.exe",
    "dwm.exe",
    "csrss.exe",
    "winlogon.exe",
    "ctfmon.exe",
    "sihost.exe",
    "taskhostw.exe",
    "RuntimeBroker.exe",
  ],
  linux: [
    "gnome-shell",
    "plasmashell",
    "kwin_x11",
    "kwin_wayland",
    "Xorg",
    "Xwayland",
    "xfwm4",
    "xfce4-panel",
  ],
};

/**
 * Remote-access, screen-sharing, virtual-camera, messaging and AI "interview
 * assistant" tools. These are the only applications that must be closed for
 * the exam, with or without a window. Matched as whole words in the
 * lowercase process name.
 */
const BLOCKED_PROCESSES = [
  "teamviewer",
  "anydesk",
  "rustdesk",
  "parsec",
  "remoting_host",
  "screenconnect",
  "connectwise",
  "vncserver",
  "vncviewer",
  "tightvnc",
  "ultravnc",
  "realvnc",
  "splashtop",
  "ammyy",
  "ultraviewer",
  "logmein",
  "obs",
  "streamlabs",
  "xsplit",
  "manycam",
  "snap camera",
  "camtwist",
  "loom",
  "screenflow",
  "bandicam",
  "zoom.us",
  "zoom",
  "discord",
  "skype",
  "msteams",
  "teams",
  "slack",
  "webex",
  "whatsapp",
  "telegram",
  "cluely",
  "interviewcoder",
  "interview coder",
  "finalround",
  "lockedin",
];

// Development only: skip kiosk mode so the UI can be worked on.
const NO_LOCKDOWN = overridable && process.env.BENCHMARQ_NO_LOCKDOWN === "1";

module.exports = {
  APP_URL,
  APP_ORIGIN: APP_URL.origin,
  EXAM_URL: new URL("/exam", APP_URL).toString(),
  // Where candidates launch exams from, opened in their browser.
  PORTAL_URL: new URL("/dashboard/my-exams", APP_URL).toString(),
  API_URL,
  PROTOCOL,
  SYSTEM_APPS: SYSTEM_APPS[process.platform] ?? [],
  BLOCKED_PROCESSES,
  NO_LOCKDOWN,
  APP_SCAN_INTERVAL_MS: 15_000,
  // Proctoring snapshots: webcam + screen every 30 s, JPEG quality 20 (0.2).
  SNAPSHOT_INTERVAL_MS: 30_000,
  SNAPSHOT_JPEG_QUALITY: 20,
  REQUEST_TIMEOUT_MS: 15_000,
};
