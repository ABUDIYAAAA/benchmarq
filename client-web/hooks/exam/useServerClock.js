"use client";

import { useEffect, useState } from "react";

// Countdowns are computed against the server's clock: the offset between
// server_time and the local clock is captured with every server payload, so
// a candidate changing their system time changes nothing.

let offsetMs = 0;

export function syncServerTime(serverTime) {
  const t = Date.parse(serverTime);
  if (!Number.isNaN(t)) offsetMs = t - Date.now();
}

export function serverNow() {
  return Date.now() + offsetMs;
}

/**
 * Seconds remaining until an ISO deadline. The value is derived during render;
 * the effect only schedules re-renders every 250ms while the timer runs.
 * When frozen (e.g. paused) the value stops at the moment of freezing.
 */
export function useRemaining(deadline, { frozen = false } = {}) {
  const [, setTick] = useState(0);
  const [frozenAt, setFrozenAt] = useState(null);

  useEffect(() => {
    if (!deadline || frozen) return undefined;
    const id = setInterval(() => setTick((t) => t + 1), 250);
    return () => clearInterval(id);
  }, [deadline, frozen]);

  if (frozen && frozenAt === null) setFrozenAt(serverNow());
  if (!frozen && frozenAt !== null) setFrozenAt(null);

  if (!deadline) return null;
  const now = frozen && frozenAt !== null ? frozenAt : serverNow();
  return Math.max(0, Math.ceil((Date.parse(deadline) - now) / 1000));
}

export function formatDuration(totalSeconds) {
  if (totalSeconds == null) return "--:--";
  const s = Math.max(0, totalSeconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`;
}
