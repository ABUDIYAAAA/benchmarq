'use strict';

const { app, session } = require('electron');
const { APP_ORIGIN } = require('./config');

const isAppUrl = (url) => {
  try {
    return new URL(url).origin === APP_ORIGIN;
  } catch {
    return false;
  }
};

// Web contents allowed to use the camera: only the hidden snapshot window.
const cameraContents = new WeakSet();

/** Grants camera (video only) to one web contents. */
function allowCamera(webContents) {
  cameraContents.add(webContents);
}

const videoOnly = (details) => (details?.mediaTypes ?? []).length > 0 && details.mediaTypes.every((t) => t === 'video');

/** Standard Electron hardening: no permissions (except the snapshot camera), downloads, popups, webviews or off-site navigation. */
function harden() {
  const ses = session.defaultSession;
  ses.setPermissionRequestHandler((wc, permission, callback, details) => {
    callback(permission === 'media' && cameraContents.has(wc) && videoOnly(details));
  });
  ses.setPermissionCheckHandler((wc, permission) => permission === 'media' && Boolean(wc) && cameraContents.has(wc));
  ses.on('will-download', (event) => event.preventDefault());

  app.on('web-contents-created', (_event, contents) => {
    contents.setWindowOpenHandler(() => ({ action: 'deny' }));
    contents.on('will-attach-webview', (event) => event.preventDefault());
    const guard = (event, url) => {
      if (!isAppUrl(url)) event.preventDefault();
    };
    contents.on('will-navigate', guard);
    contents.on('will-redirect', guard);
  });

  app.on('certificate-error', (event, _wc, _url, _error, _cert, callback) => callback(false));
}

module.exports = { harden, allowCamera };
