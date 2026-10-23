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

/** Standard Electron hardening: no permissions, downloads, popups, webviews or off-site navigation. */
function harden() {
  const ses = session.defaultSession;
  ses.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
  ses.setPermissionCheckHandler(() => false);
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

module.exports = { harden };
