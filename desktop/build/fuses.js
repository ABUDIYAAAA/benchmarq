'use strict';

// Flips Electron fuses on the packaged binary so it cannot be repurposed:
// no ELECTRON_RUN_AS_NODE, no NODE_OPTIONS, no --inspect, app code only from
// an integrity-checked asar, and cookies encrypted at rest.
const path = require('node:path');
const { flipFuses, FuseVersion, FuseV1Options } = require('@electron/fuses');

module.exports = async function afterPack(context) {
  const { electronPlatformName, appOutDir, packager } = context;
  const name = packager.appInfo.productFilename;
  const binary = {
    darwin: path.join(appOutDir, `${name}.app`, 'Contents', 'MacOS', name),
    win32: path.join(appOutDir, `${name}.exe`),
    linux: path.join(appOutDir, packager.executableName),
  }[electronPlatformName];

  await flipFuses(binary, {
    version: FuseVersion.V1,
    resetAdHocDarwinSignature: electronPlatformName === 'darwin',
    [FuseV1Options.RunAsNode]: false,
    [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
    [FuseV1Options.EnableNodeCliInspectArguments]: false,
    [FuseV1Options.EnableCookieEncryption]: true,
    [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]: true,
    [FuseV1Options.OnlyLoadAppFromAsar]: true,
    [FuseV1Options.LoadBrowserProcessSpecificV8Snapshot]: false,
    [FuseV1Options.GrantFileProtocolExtraPrivileges]: false,
  });
};
