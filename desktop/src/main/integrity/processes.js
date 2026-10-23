'use strict';

const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const path = require('node:path');
const { ALLOWED_APPS, SYSTEM_APPS, BLOCKED_PROCESSES } = require('../config');

const run = promisify(execFile);
const EXEC_OPTS = { timeout: 10_000, maxBuffer: 8 * 1024 * 1024, windowsHide: true };

/**
 * @typedef {{ pid: number, name: string, path: string, windowed: boolean }} Proc
 * @typedef {{ pid: number, name: string, reason: 'not_allowed' | 'blocked' }} Offender
 */

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

/** Lists the current user's processes, flagging those that are GUI apps. */
async function listProcesses() {
  switch (process.platform) {
    case 'darwin':
      return listDarwin();
    case 'win32':
      return listWindows();
    default:
      return listLinux();
  }
}

// macOS: `ps` needs no extra permissions (unlike System Events scripting).
// Every process inside an app bundle is attributed to the outermost bundle,
// so helpers ("Discord Helper (Renderer)") are reported and killed with
// their app. The app counts as windowed through its main executable,
// X.app/Contents/MacOS/X.
async function listDarwin() {
  const uid = process.getuid();
  const { stdout } = await run('/bin/ps', ['-axo', 'pid=,uid=,comm='], EXEC_OPTS);
  const out = [];
  for (const line of stdout.split('\n')) {
    const m = line.match(/^\s*(\d+)\s+(\d+)\s+(.+)$/);
    if (!m || Number(m[2]) !== uid) continue;
    const exe = m[3].trim();
    const outer = exe.match(/^(.*?\/([^/]+)\.app)\//);
    const mainExe = outer && new RegExp(`^${escape(outer[1])}/Contents/MacOS/[^/]+$`).test(exe);
    out.push({
      pid: Number(m[1]),
      name: outer ? outer[2] : path.basename(exe),
      path: exe,
      windowed: Boolean(mainExe) && !exe.startsWith('/System/'),
    });
  }
  return out;
}

async function listWindows() {
  const script =
    "Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.SessionId -eq (Get-Process -Id $PID).SessionId } | " +
    "Select-Object Id, ProcessName, @{n='Path';e={$_.Path}}, @{n='Win';e={$_.MainWindowHandle -ne 0}} | ConvertTo-Json -Compress";
  const { stdout } = await run('powershell.exe', ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-Command', script], EXEC_OPTS);
  const rows = JSON.parse(stdout || '[]');
  return (Array.isArray(rows) ? rows : [rows]).map((r) => ({
    pid: r.Id,
    name: `${r.ProcessName}.exe`,
    path: r.Path ?? '',
    windowed: Boolean(r.Win),
  }));
}

// Linux has no portable notion of "app with a window"; wmctrl (if present)
// lists window owners, otherwise only the blocklist is enforced.
async function listLinux() {
  const uid = process.getuid();
  const { stdout } = await run('ps', ['-eo', 'pid=,uid=,comm='], EXEC_OPTS);
  const windowedPids = new Set();
  try {
    const { stdout: wm } = await run('wmctrl', ['-lp'], EXEC_OPTS);
    for (const line of wm.split('\n')) {
      const pid = Number(line.trim().split(/\s+/)[2]);
      if (pid > 0) windowedPids.add(pid);
    }
  } catch {
    // wmctrl unavailable
  }
  return stdout
    .split('\n')
    .map((line) => line.match(/^\s*(\d+)\s+(\d+)\s+(.+)$/))
    .filter((m) => m && Number(m[2]) === uid)
    .map((m) => ({ pid: Number(m[1]), name: m[3].trim(), path: '', windowed: windowedPids.has(Number(m[1])) }));
}

// ---------------------------------------------------------------------------
// Policy
// ---------------------------------------------------------------------------

const normalize = (name) => name.toLowerCase().replace(/\.(exe|app)$/, '').trim();

function matchesAny(name, list) {
  const n = normalize(name);
  return list.some((entry) => normalize(entry) === n);
}

/** Whole-word match so "obs" catches "OBS Studio" but not "Obsidian". */
function isBlocked(name) {
  const n = normalize(name);
  return BLOCKED_PROCESSES.some((entry) => new RegExp(`(^|[^a-z0-9])${escape(entry)}([^a-z0-9]|$)`).test(n));
}

function escape(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Finds processes that must be closed before (and stay closed during) the
 * exam: blocklisted tools regardless of windows, and any windowed app that is
 * neither allowed nor part of the OS. ownPids are this app's own processes.
 *
 * @param {Proc[]} procs
 * @param {Set<number>} ownPids
 * @returns {Offender[]}
 */
function findOffenders(procs, ownPids) {
  const seen = new Map();
  for (const p of procs) {
    if (ownPids.has(p.pid) || matchesAny(p.name, SYSTEM_APPS)) continue;
    let reason = null;
    if (isBlocked(p.name)) reason = 'blocked';
    else if (p.windowed && !matchesAny(p.name, ALLOWED_APPS)) reason = 'not_allowed';
    if (!reason) continue;

    // Report one row per app, but remember every PID to terminate.
    const key = normalize(p.name);
    const row = seen.get(key) ?? { name: p.name, reason, pids: [] };
    row.pids.push(p.pid);
    if (reason === 'blocked') row.reason = 'blocked';
    seen.set(key, row);
  }
  return [...seen.values()];
}

// ---------------------------------------------------------------------------
// Termination
// ---------------------------------------------------------------------------

const alive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Asks each process to exit, then force-kills whatever is still running after
 * a grace period. Returns the names of apps that could not be closed.
 */
async function terminate(offenders, ownPids, graceMs = 3000) {
  const pids = offenders.flatMap((o) => o.pids).filter((pid) => !ownPids.has(pid) && pid !== process.pid);

  for (const pid of pids) {
    if (process.platform === 'win32') {
      await run('taskkill', ['/PID', String(pid), '/T'], EXEC_OPTS).catch(() => {});
    } else {
      try {
        process.kill(pid, 'SIGTERM');
      } catch {
        // already gone or not ours
      }
    }
  }

  const deadline = Date.now() + graceMs;
  while (Date.now() < deadline && pids.some(alive)) await sleep(200);

  for (const pid of pids.filter(alive)) {
    if (process.platform === 'win32') {
      await run('taskkill', ['/PID', String(pid), '/T', '/F'], EXEC_OPTS).catch(() => {});
    } else {
      try {
        process.kill(pid, 'SIGKILL');
      } catch {
        // ignore
      }
    }
  }
  await sleep(300);

  return offenders.filter((o) => o.pids.some(alive)).map((o) => o.name);
}

module.exports = { listProcesses, findOffenders, terminate, isBlocked };
