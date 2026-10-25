"use client";

import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { ATTEMPT_ENDED, getBridge, invoke } from "@/lib/exam/bridge";
import { syncServerTime } from "@/hooks/exam/useServerClock";

/**
 * Single source of truth for the exam window. It mirrors what the desktop
 * app's main process knows (session and attempt) and derives
 * the screen to show:
 *
 *   unsupported  -> not running inside Benchmarq Exam
 *   loading      -> waiting for the bridge
 *   no_session   -> opened without a valid launch link
 *   system_check -> other applications must be closed first
 *   waiting     -> waiting room until the exam can start (or resume)
 *   exam        -> attempt in progress
 *   ended       -> attempt finished
 */
// The bridge is injected before any page script runs and never changes.
const subscribeNever = () => () => {};

export function useExamRuntime() {
  // undefined while server-rendering/hydrating, null outside the desktop app.
  const bridge = useSyncExternalStore(subscribeNever, getBridge, () => undefined);
  const [boot, setBoot] = useState(null);
  const [system, setSystem] = useState(null);
  const [state, setState] = useState(null);
  const [attempt, setAttempt] = useState(null);
  const [entered, setEntered] = useState(false);
  const [online, setOnline] = useState(true);
  const [invalid, setInvalid] = useState(null);
  const [violation, setViolation] = useState(null);
  const violationTimer = useRef(null);

  const applyState = useCallback((s) => {
    if (!s) return;
    syncServerTime(s.server_time);
    setState(s);
    setAttempt(s.attempt ?? null);
  }, []);

  const applyAttempt = useCallback((a) => {
    if (!a) return;
    if (a.clock?.server_time) syncServerTime(a.clock.server_time);
    setAttempt(a);
  }, []);

  useEffect(() => {
    if (!bridge) return undefined;
    let cancelled = false;

    invoke(bridge.session.bootstrap).then((b) => {
      if (cancelled) return;
      setBoot(b);
      setSystem(b.system);
      applyState(b.state);
    });

    const offs = [
      bridge.on("runtime:state", applyState),
      bridge.on("runtime:attempt", applyAttempt),
      bridge.on("runtime:ended", applyAttempt),
      bridge.on("runtime:connection", (c) => setOnline(c.online)),
      bridge.on("runtime:invalid", (err) => setInvalid(err)),
      bridge.on("runtime:violation", (v) => {
        setViolation(v);
        clearTimeout(violationTimer.current);
        violationTimer.current = setTimeout(() => setViolation(null), 6000);
      }),
    ];
    return () => {
      cancelled = true;
      offs.forEach((off) => off());
      clearTimeout(violationTimer.current);
    };
  }, [bridge, applyState, applyAttempt]);

  const status = attempt?.status;
  const phase = (() => {
    if (bridge === undefined) return "loading";
    if (bridge === null) return "unsupported";
    if (!boot) return "loading";
    if (status && ATTEMPT_ENDED.has(status)) return "ended";
    if (invalid || !state) return "no_session";
    if (system && !system.clear) return "system_check";
    if (status === "in_progress" && entered) return "exam";
    return "waiting";
  })();

  const actions = useMemo(() => {
    if (!bridge) return {};
    const rt = bridge.runtime;
    return {
      refresh: async () => applyState(await invoke(rt.refresh)),
      start: async () => {
        applyState(await invoke(rt.start));
        setEntered(true);
      },
      resumeAttempt: async () => {
        applyState(await invoke(rt.resumeAttempt));
        setEntered(true);
      },
      enterSection: async (id) => applyAttempt(await invoke(rt.enterSection, id)),
      finishSection: async (id) => applyAttempt(await invoke(rt.finishSection, id)),
      pause: async () => applyAttempt(await invoke(rt.pause)),
      resume: async () => applyAttempt(await invoke(rt.resume)),
      submit: async () => applyAttempt(await invoke(rt.submit)),
      question: () => invoke(rt.question),
      navigate: (position) => invoke(rt.navigate, position),
      saveResponse: (input) => invoke(rt.saveResponse, input),
      checkSystem: async () => setSystem(await invoke(bridge.system.check)),
      closeApps: async () => setSystem(await invoke(bridge.system.closeApps)),
      openPortal: () => invoke(bridge.app.openPortal),
      requestCamera: () => invoke(bridge.system.requestCamera),
      enterFullscreen: () => invoke(bridge.app.enterFullscreen),
      quit: () => invoke(bridge.app.quit),
    };
  }, [bridge, applyState, applyAttempt]);

  return {
    bridge,
    boot,
    system,
    phase,
    state,
    attempt,
    setAttempt,
    rules: state?.rules ?? null,
    online,
    invalid,
    violation,
    actions,
  };
}
