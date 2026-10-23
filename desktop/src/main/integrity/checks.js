'use strict';

const { app } = require('electron');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const inspector = require('node:inspector');
const processes = require('./processes');

const run = promisify(execFile);
const EXEC_OPTS = { timeout: 8000, windowsHide: true };

const DEBUG_SWITCHES = ['--inspect', '--inspect-brk', '--remote-debugging-port', '--remote-debugging-pipe', '--js-flags'];

/**
 * Reports whether the app was started in a way that allows code injection:
 * debugger or Node switches, an open inspector, or Node mode env variables.
 * Packaged builds also have these disabled by Electron fuses (build/fuses.js),
 * and the fuses make Electron refuse to load a modified app.asar.
 */
function tampered() {
  const argv = [...process.argv, ...process.execArgv];
  if (argv.some((a) => DEBUG_SWITCHES.some((s) => a.startsWith(s)))) return true;
  if (DEBUG_SWITCHES.some((s) => app.commandLine.hasSwitch(s.slice(2)))) return true;
  if (inspector.url()) return true;
  return Boolean(process.env.ELECTRON_RUN_AS_NODE || process.env.NODE_OPTIONS);
}

async function virtualMachine() {
  try {
    if (process.platform === 'darwin') {
      const { stdout } = await run('/usr/sbin/sysctl', ['-n', 'kern.hv_vmm_present'], EXEC_OPTS);
      return stdout.trim() === '1';
    }
    if (process.platform === 'win32') {
      const { stdout } = await run(
        'powershell.exe',
        ['-NoProfile', '-NonInteractive', '-Command', 'Get-CimInstance Win32_ComputerSystem | Select-Object Manufacturer,Model | ConvertTo-Json -Compress'],
        EXEC_OPTS,
      );
      return /virtual|vmware|virtualbox|kvm|qemu|parallels|xen|hyper-v/i.test(stdout);
    }
    const { stdout } = await run('systemd-detect-virt', [], EXEC_OPTS).catch((e) => ({ stdout: e.stdout ?? 'none' }));
    return stdout.trim() !== '' && stdout.trim() !== 'none';
  } catch {
    return false;
  }
}

function ownPids() {
  const pids = new Set(app.getAppMetrics().map((m) => m.pid));
  pids.add(process.pid);
  return pids;
}

/** Applications that must be closed, grouped per app with all their PIDs. */
async function disallowedApps() {
  const procs = await processes.listProcesses().catch(() => []);
  return processes.findOffenders(procs, ownPids());
}

function closeApps(offenders) {
  return processes.terminate(offenders, ownPids());
}

module.exports = { tampered, virtualMachine, disallowedApps, closeApps };
