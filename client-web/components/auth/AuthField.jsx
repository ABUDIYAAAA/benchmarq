"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import OutlinedInput from "@mui/material/OutlinedInput";
import InputAdornment from "@mui/material/InputAdornment";
import IconButton from "@mui/material/IconButton";

export default function AuthField({
  id,
  name,
  label,
  type = "text",
  value,
  onChange,
  placeholder,
  required = false,
  error,
  helperText,
  disabled = false,
  autoComplete,
  autoFocus = false,
}) {
  const [showPassword, setShowPassword] = useState(false);
  const isPassword = type === "password";
  const inputType = isPassword ? (showPassword ? "text" : "password") : type;

  return (
    <Box sx={{ width: "100%", mb: 2 }}>
      {label && (
        <Box
          component="label"
          htmlFor={id || name}
          sx={{
            display: "block",
            fontSize: "0.8125rem",
            fontWeight: 500,
            color: "#09090b",
            mb: 0.75,
            letterSpacing: "-0.005em",
          }}
        >
          {label} {required && <span style={{ color: "#71717a" }}>*</span>}
        </Box>
      )}

      <OutlinedInput
        id={id || name}
        name={name}
        type={inputType}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        disabled={disabled}
        required={required}
        autoComplete={autoComplete}
        autoFocus={autoFocus}
        fullWidth
        error={Boolean(error)}
        endAdornment={
          isPassword ? (
            <InputAdornment position="end">
              <IconButton
                aria-label={showPassword ? "Hide password" : "Show password"}
                onClick={() => setShowPassword((prev) => !prev)}
                edge="end"
                size="small"
                sx={{
                  color: "#71717a",
                  "&:hover": { color: "#09090b" },
                  p: 0.5,
                }}
              >
                {showPassword ? (
                  /* Clean Eye-off SVG */
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.75"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" />
                    <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" />
                    <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" />
                    <line x1="2" y1="2" x2="22" y2="22" />
                  </svg>
                ) : (
                  /* Clean Eye SVG */
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.75"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                )}
              </IconButton>
            </InputAdornment>
          ) : null
        }
      />

      {(error || helperText) && (
        <Typography
          sx={{
            fontSize: "0.75rem",
            color: error ? "#09090b" : "#71717a",
            fontWeight: error ? 500 : 400,
            mt: 0.5,
            pl: 0.25,
          }}
        >
          {error || helperText}
        </Typography>
      )}
    </Box>
  );
}
