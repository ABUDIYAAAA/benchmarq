// Access to the desktop app's preload bridge. The exam route only works
// inside Benchmarq Exam: there is no fallback to direct API calls, because the
// session token never leaves the desktop app's main process.

export function getBridge() {
  if (typeof window === "undefined") return null;
  return window.benchmarq ?? null;
}

/** Normalizes bridge rejections (plain objects) into Error instances. */
export function toError(err) {
  if (err instanceof Error) return err;
  const e = new Error(err?.message || "Something went wrong");
  e.status = err?.status ?? 0;
  e.fields = err?.fields ?? null;
  return e;
}

/** Calls a bridge method and rethrows failures as Errors. */
export async function invoke(fn, ...args) {
  try {
    return await fn(...args);
  } catch (err) {
    throw toError(err);
  }
}

export const ATTEMPT_ENDED = new Set(["submitted", "auto_submitted", "disqualified", "abandoned", "evaluated"]);

export const END_REASON_TEXT = {
  submitted: "You submitted the exam.",
  time_expired: "Time ran out and your answers were submitted automatically.",
  disconnected: "The exam ended because the app was disconnected for too long.",
  violation_limit: "The exam ended because the proctoring limits were exceeded.",
  exam_closed: "The exam window closed.",
};

export const VIOLATION_TEXT = {
  fullscreen_exit: "You left full-screen mode. This has been recorded.",
  tab_switch: "The exam window lost focus. This has been recorded.",
};
