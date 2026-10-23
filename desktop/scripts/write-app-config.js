'use strict';

// Writes src/main/app-config.json, the URLs a packaged build talks to.
// Packaged apps ignore environment variables at runtime, so the URLs must be
// fixed at build time:
//
//   BENCHMARQ_APP_URL=https://app.example.com BENCHMARQ_API_URL=https://api.example.com/api/v1 npm run dist:mac
//
// Without the variables the production defaults in config.js apply.
const fs = require('node:fs');
const path = require('node:path');

const out = path.join(__dirname, '..', 'src', 'main', 'app-config.json');
const config = {};
for (const [env, key] of [
  ['BENCHMARQ_APP_URL', 'appUrl'],
  ['BENCHMARQ_API_URL', 'apiUrl'],
]) {
  const value = process.env[env];
  if (!value) continue;
  new URL(value); // fail the build on a malformed URL
  config[key] = value;
}
fs.writeFileSync(out, JSON.stringify(config, null, 2) + '\n');
console.log(`app-config.json: ${Object.keys(config).length ? JSON.stringify(config) : 'production defaults'}`);
