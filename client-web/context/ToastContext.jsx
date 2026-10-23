"use client";

import { createContext, useContext, useState, useCallback, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import ButtonBase from "@mui/material/ButtonBase";

const ToastContext = createContext(null);

export function ToastProvider({ children }) {
  const [toast, setToast] = useState(null);
  const timerRef = useRef(null);

  const dismiss = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setToast(null);
  }, []);

  const showToast = useCallback(
    (message, type = "success", duration = 4000) => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }

      setToast({ message, type, id: Date.now() });

      if (duration > 0) {
        timerRef.current = setTimeout(() => {
          setToast(null);
          timerRef.current = null;
        }, duration);
      }
    },
    []
  );

  const showSuccess = useCallback((msg, duration) => showToast(msg, "success", duration), [showToast]);
  const showError = useCallback((msg, duration) => showToast(msg, "error", duration), [showToast]);
  const showInfo = useCallback((msg, duration) => showToast(msg, "info", duration), [showToast]);

  return (
    <ToastContext.Provider value={{ showToast, showSuccess, showError, showInfo, dismiss }}>
      {children}

      {/* Floating Toast notification */}
      {toast && (
        <Box
          key={toast.id}
          role="status"
          aria-live="polite"
          sx={{
            position: "fixed",
            bottom: { xs: 20, sm: 28 },
            right: { xs: 20, sm: 28 },
            zIndex: 3000,
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            px: 2,
            py: 1.25,
            minWidth: 280,
            maxWidth: 440,
            backgroundColor: "#09090b",
            color: "#fafafa",
            border: "1px solid rgba(255, 255, 255, 0.14)",
            borderRadius: "8px",
            boxShadow: "0 12px 32px rgba(0, 0, 0, 0.28)",
            animation: "toastIn 160ms cubic-bezier(0.16, 1, 0.3, 1) forwards",
            "@keyframes toastIn": {
              "0%": {
                opacity: 0,
                transform: "translateY(8px) scale(0.98)",
              },
              "100%": {
                opacity: 1,
                transform: "translateY(0) scale(1)",
              },
            },
          }}
        >
          {/* Micro Icon */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 18,
              height: 18,
              borderRadius: "50%",
              flexShrink: 0,
              color: toast.type === "error" ? "#f87171" : "#ffffff",
            }}
          >
            {toast.type === "error" ? (
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            ) : toast.type === "info" ? (
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="16" x2="12" y2="12" />
                <line x1="12" y1="8" x2="12.01" y2="8" />
              </svg>
            ) : (
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            )}
          </Box>

          <Typography
            sx={{
              fontSize: "0.8125rem",
              fontWeight: 500,
              lineHeight: 1.4,
              color: "#fafafa",
              flex: 1,
            }}
          >
            {toast.message}
          </Typography>

          {/* Dismiss button */}
          <ButtonBase
            onClick={dismiss}
            aria-label="Dismiss notification"
            sx={{
              color: "#a1a1aa",
              p: 0.5,
              borderRadius: "4px",
              transition: "color 120ms ease-out, background-color 120ms ease-out",
              "&:hover": {
                color: "#fafafa",
                backgroundColor: "rgba(255, 255, 255, 0.08)",
              },
            }}
          >
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </ButtonBase>
        </Box>
      )}
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return context;
}
