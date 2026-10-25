'use strict';

const { EventEmitter } = require('node:events');
const { API_URL, REQUEST_TIMEOUT_MS } = require('./config');
const { deviceId } = require('./device');
const store = require('./store');

const SESSION_KEY = 'session';
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const LAUNCH_CODE = /^[A-Za-z0-9_-]{8,128}$/;
const EVENT_TYPES = new Set(['fullscreen_exit', 'tab_switch', 'clipboard_action', 'dev_tools_opened', 'multiple_faces', 'no_face', 'audio_detected']);
const ENDED = new Set(['submitted', 'auto_submitted', 'disqualified', 'abandoned', 'evaluated']);

class RuntimeError extends Error {
  constructor(status, message, fields) {
    super(message);
    this.status = status;
    this.fields = fields ?? null;
  }

  /** Serializable shape sent to the renderer. */
  toJSON() {
    return { status: this.status, message: this.message, fields: this.fields };
  }
}

/**
 * Client for the server's /runtime API. The session token lives only in the
 * main process (and, encrypted, on disk so the app can resume after the
 * close-apps restart). The renderer never sees it: it calls named methods
 * over IPC, and each method maps to exactly one endpoint.
 *
 * Events:
 *   state      RuntimeState from the server
 *   attempt    AttemptView (heartbeats, section changes, pause/resume)
 *   connection { online: boolean }
 *   ended      AttemptView of a finished attempt
 *   invalid    the session was rejected; the candidate must relaunch
 */
class Runtime extends EventEmitter {
  constructor() {
    super();
    this.session = store.read(SESSION_KEY); // { token, expiresAt }
    this.state = null;
    this.online = true;
    this.heartbeatTimer = null;
  }

  hasSession() {
    return Boolean(this.session && Date.parse(this.session.expiresAt) > Date.now());
  }

  async openSession(launchCode) {
    if (!LAUNCH_CODE.test(launchCode)) throw new RuntimeError(400, 'This launch link is malformed');
    const grant = await this.#request('POST', '/runtime/sessions', { launch_code: launchCode, device_id: deviceId() }, { auth: false });
    this.session = { token: grant.token, expiresAt: grant.expires_at };
    store.write(SESSION_KEY, this.session);
    this.#setState(grant.state);
    return grant.state;
  }

  clearSession() {
    this.stopHeartbeat();
    this.session = null;
    this.state = null;
    store.remove(SESSION_KEY);
  }

  attemptRunning() {
    return this.state?.attempt?.status === 'in_progress';
  }

  // ----- Endpoints ---------------------------------------------------------

  async refresh() {
    return this.#setState(await this.#request('GET', '/runtime/state'));
  }

  async start() {
    const state = this.#setState(await this.#request('POST', '/runtime/start'));
    this.startHeartbeat();
    return state;
  }

  async heartbeat() {
    return this.#setAttempt(await this.#request('POST', '/runtime/heartbeat'));
  }

  async enterSection(sectionId) {
    return this.#setAttempt(await this.#request('POST', `/runtime/sections/${uuid(sectionId)}/enter`));
  }

  async finishSection(sectionId) {
    return this.#setAttempt(await this.#request('POST', `/runtime/sections/${uuid(sectionId)}/finish`));
  }

  currentQuestion() {
    return this.#request('GET', '/runtime/question');
  }

  navigate(position) {
    if (!Number.isInteger(position) || position < 0 || position > 10_000) throw new RuntimeError(400, 'Invalid question position');
    return this.#request('POST', '/runtime/question/navigate', { position });
  }

  saveResponse(input) {
    return this.#request('PUT', '/runtime/question/response', sanitizeResponse(input));
  }

  async pause() {
    return this.#setAttempt(await this.#request('POST', '/runtime/pause'));
  }

  async resume() {
    return this.#setAttempt(await this.#request('POST', '/runtime/resume'));
  }

  async submit() {
    return this.#setAttempt(await this.#request('POST', '/runtime/submit'));
  }

  /**
   * Uploads proctoring snapshots (JPEG Buffers). Failures are swallowed: a
   * missed snapshot must never interrupt the exam.
   */
  async uploadSnapshots({ webcam, screen }) {
    if (!this.attemptRunning()) return null;
    const form = new FormData();
    if (webcam) form.append('webcam', new Blob([webcam], { type: 'image/jpeg' }), 'webcam.jpg');
    if (screen) form.append('screen', new Blob([screen], { type: 'image/jpeg' }), 'screen.jpg');
    try {
      return await this.#request('POST', '/runtime/snapshots', form);
    } catch (err) {
      console.warn('snapshots: upload failed', err?.status, err?.message);
      return null;
    }
  }

  /** Reports a proctoring event. Failures are swallowed: they must never break the exam UI. */
  async reportEvent(type, details = {}) {
    if (!EVENT_TYPES.has(type) || !this.attemptRunning()) return null;
    const body = { type, client_time: new Date().toISOString(), details: limitDetails(details) };
    if (type === 'tab_switch' && Number.isInteger(details.duration_ms)) body.duration_ms = Math.min(details.duration_ms, 86_400_000);
    try {
      const result = await this.#request('POST', '/runtime/events', body);
      if (result?.ended) await this.refresh();
      return result;
    } catch {
      return null;
    }
  }

  // ----- Heartbeat ---------------------------------------------------------

  startHeartbeat() {
    this.stopHeartbeat();
    const seconds = this.state?.rules?.heartbeat_interval_seconds ?? 15;
    this.heartbeatTimer = setInterval(() => {
      this.heartbeat().catch(() => {});
    }, Math.max(5, seconds) * 1000);
  }

  stopHeartbeat() {
    clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  // ----- Internals ---------------------------------------------------------

  #setState(state) {
    this.state = state;
    this.emit('state', state);
    this.#checkEnded(state?.attempt);
    if (this.attemptRunning() && !this.heartbeatTimer) this.startHeartbeat();
    return state;
  }

  #setAttempt(attempt) {
    if (this.state) this.state = { ...this.state, attempt };
    this.emit('attempt', attempt);
    this.#checkEnded(attempt);
    return attempt;
  }

  #checkEnded(attempt) {
    if (attempt && ENDED.has(attempt.status)) {
      this.stopHeartbeat();
      store.remove(SESSION_KEY); // the session cannot be resumed once the attempt is over
      this.emit('ended', attempt);
    }
  }

  #setOnline(online) {
    if (this.online !== online) {
      this.online = online;
      this.emit('connection', { online });
    }
  }

  async #request(method, path, body, { auth = true } = {}) {
    const headers = { Accept: 'application/json', 'X-Device-ID': deviceId() };
    const multipart = body instanceof FormData;
    if (body !== undefined && !multipart) headers['Content-Type'] = 'application/json';
    if (auth) {
      if (!this.session) throw new RuntimeError(401, 'No exam session; launch the exam from the portal');
      headers.Authorization = `Bearer ${this.session.token}`;
    }

    let res;
    try {
      res = await fetch(API_URL + path, {
        method,
        headers,
        body: body === undefined ? undefined : multipart ? body : JSON.stringify(body),
        signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
        redirect: 'error',
        cache: 'no-store',
      });
    } catch {
      this.#setOnline(false);
      throw new RuntimeError(0, 'Cannot reach the exam server. Check your connection; your answers are saved.');
    }
    this.#setOnline(true);

    const payload = await res.json().catch(() => ({}));
    if (!res.ok || payload.success === false) {
      const err = new RuntimeError(res.status, payload.message || `Request failed (${res.status})`, payload.errors);
      if (res.status === 401 && auth) {
        this.clearSession();
        this.emit('invalid', err.toJSON());
      }
      throw err;
    }
    return payload.data;
  }
}

function uuid(value) {
  if (typeof value !== 'string' || !UUID.test(value)) throw new RuntimeError(400, 'Invalid identifier');
  return value;
}

/** Builds a SaveResponseRequest from untrusted renderer input, field by field. */
function sanitizeResponse(input) {
  if (!input || typeof input !== 'object') throw new RuntimeError(400, 'Invalid response');
  const out = { question_id: uuid(input.question_id) };
  if (input.selected_option_id !== undefined) out.selected_option_id = uuid(input.selected_option_id);
  if (input.selected_option_ids !== undefined) {
    if (!Array.isArray(input.selected_option_ids) || input.selected_option_ids.length > 26) throw new RuntimeError(400, 'Invalid selection');
    out.selected_option_ids = [...new Set(input.selected_option_ids.map(uuid))];
  }
  if (input.text !== undefined) {
    if (typeof input.text !== 'string' || input.text.length > 500_000) throw new RuntimeError(400, 'Answer is too long');
    out.text = input.text;
  }
  if (input.boolean !== undefined) out.boolean = Boolean(input.boolean);
  if (input.clear === true) out.clear = true;
  if (input.flagged !== undefined) out.flagged = Boolean(input.flagged);
  return out;
}

/** Keeps event details small and flat so the renderer cannot ship arbitrary data. */
function limitDetails(details) {
  const out = {};
  for (const [k, v] of Object.entries(details ?? {}).slice(0, 10)) {
    if (['string', 'number', 'boolean'].includes(typeof v)) out[String(k).slice(0, 40)] = typeof v === 'string' ? v.slice(0, 200) : v;
  }
  return out;
}

module.exports = { Runtime, RuntimeError, sanitizeResponse };
