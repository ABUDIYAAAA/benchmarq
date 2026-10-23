'use strict';

// Preload for the hidden camera window: lets the page answer capture requests
// from the main process and nothing else.

const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('capture', {
  onTake: (callback) => {
    ipcRenderer.on('capture:take', (_event, id) => callback(id));
  },
  reply: (id, buffer, error) => {
    ipcRenderer.send('capture:frame', { id, buffer, error });
  },
});
