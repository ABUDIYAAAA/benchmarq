'use strict';

// Flips Electron fuses on the packaged binary so it cannot be repurposed:
// no ELECTRON_RUN_AS_NODE, no NODE_OPTIONS, no --inspect, app code only from
// an integrity-checked asar, and cookies encrypted at rest.
const fs = require('node:fs');
const path = require('node:path');
const { flipFuses, FuseVersion, FuseV1Options } = require('@electron/fuses');

/** Finds the Electron executable in the packed output. */
function findBinary(appOutDir, platform) {
  const entries = fs.readdirSync(appOutDir);
  if (platform === 'darwin') {
    const bundle = entries.find((e) => e.endsWith('.app'));
    const macosDir = path.join(appOutDir, bundle, 'Contents', 'MacOS');
    return path.join(macosDir, fs.readdirSync(macosDir)[0]);
  }
  if (platform === 'win32') {
    return path.join(appOutDir, entries.find((e) => e.endsWith('.exe') && !/uninstall/i.test(e)));
  }
  // Linux: the executable is the only top-level file with the exec bit set
  // that is not a shared library or helper.
  const exe = entries.find((e) => {
    const full = path.join(appOutDir, e);
    const stat = fs.statSync(full);
    return stat.isFile() && stat.mode & 0o111 && !e.includes('.') && e !== 'chrome-sandbox' && e !== 'chrome_crashpad_handler';
  });
  return path.join(appOutDir, exe);
}

module.exports = async function afterPack(context) {
  const binary = findBinary(context.appOutDir, context.electronPlatformName);
  await flipFuses(binary, {
    version: FuseVersion.V1,
    resetAdHocDarwinSignature: context.electronPlatformName === 'darwin',
    [FuseV1Options.RunAsNode]: false,
    [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
    [FuseV1Options.EnableNodeCliInspectArguments]: false,
    [FuseV1Options.EnableCookieEncryption]: true,
    [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]: true,
    [FuseV1Options.OnlyLoadAppFromAsar]: true,
    [FuseV1Options.LoadBrowserProcessSpecificV8Snapshot]: false,
    [FuseV1Options.GrantFileProtocolExtraPrivileges]: false,
  });
  console.log(`  • fuses flipped  binary=${path.relative(context.appOutDir, binary)}`);
};
