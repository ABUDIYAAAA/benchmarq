import { createTheme } from "@mui/material/styles";

export const theme = createTheme({
  palette: {
    mode: "light",
    primary: {
      main: "#09090b",
      light: "#27272a",
      dark: "#000000",
      contrastText: "#ffffff",
    },
    secondary: {
      main: "#71717a",
      contrastText: "#ffffff",
    },
    background: {
      default: "#ffffff",
      paper: "#ffffff",
    },
    text: {
      primary: "#09090b",
      secondary: "#71717a",
      disabled: "#a1a1aa",
    },
    divider: "#e4e4e7",
    action: {
      hover: "#f4f4f5",
      selected: "#f4f4f5",
      disabled: "#a1a1aa",
      disabledBackground: "#f4f4f5",
    },
    error: {
      main: "#18181b", // Monochrome alert approach with clear border/contrast
    },
  },
  typography: {
    fontFamily: "var(--font-geist-sans), -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    h1: {
      fontSize: "1.75rem",
      fontWeight: 600,
      letterSpacing: "-0.02em",
      color: "#09090b",
    },
    h2: {
      fontSize: "1.5rem",
      fontWeight: 600,
      letterSpacing: "-0.015em",
      color: "#09090b",
    },
    h3: {
      fontSize: "1.25rem",
      fontWeight: 600,
      letterSpacing: "-0.01em",
      color: "#09090b",
    },
    body1: {
      fontSize: "0.875rem",
      lineHeight: 1.5,
      color: "#09090b",
    },
    body2: {
      fontSize: "0.8125rem",
      lineHeight: 1.45,
      color: "#71717a",
    },
    button: {
      textTransform: "none",
      fontWeight: 500,
      fontSize: "0.875rem",
      letterSpacing: "0",
    },
  },
  shape: {
    borderRadius: 6,
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          backgroundColor: "#ffffff",
          color: "#09090b",
        },
      },
    },
    MuiButton: {
      defaultProps: {
        disableElevation: true,
      },
      styleOverrides: {
        root: {
          borderRadius: 6,
          padding: "8px 16px",
          transition: "transform 120ms ease-out, background-color 140ms ease-out, border-color 140ms ease-out",
          "&:active": {
            transform: "scale(0.985)",
          },
        },
        containedPrimary: {
          backgroundColor: "#09090b",
          color: "#ffffff",
          "&:hover": {
            backgroundColor: "#27272a",
          },
          "&.Mui-disabled": {
            backgroundColor: "#e4e4e7",
            color: "#a1a1aa",
          },
        },
        outlinedPrimary: {
          borderColor: "#e4e4e7",
          color: "#09090b",
          backgroundColor: "#ffffff",
          "&:hover": {
            borderColor: "#09090b",
            backgroundColor: "#f4f4f5",
          },
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          borderRadius: 6,
          backgroundColor: "#ffffff",
          transition: "border-color 140ms ease-out",
          "& .MuiOutlinedInput-notchedOutline": {
            borderColor: "#e4e4e7",
            borderWidth: "1px",
          },
          "&:hover .MuiOutlinedInput-notchedOutline": {
            borderColor: "#a1a1aa",
          },
          "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
            borderColor: "#09090b",
            borderWidth: "1.5px",
          },
        },
        input: {
          fontSize: "0.875rem",
          padding: "10px 14px",
          color: "#09090b",
        },
        // Match the single-line padding so labels line up in text areas too.
        multiline: {
          padding: "10px 14px",
        },
      },
    },
    MuiInputLabel: {
      styleOverrides: {
        root: {
          fontSize: "0.875rem",
          color: "#71717a",
          // The resting label is centred for MUI's 56px field; inputs here are
          // 40px tall (10px padding), so move it up to sit on the text line.
          "&.MuiInputLabel-outlined:not(.MuiInputLabel-shrink)": {
            transform: "translate(14px, 10px) scale(1)",
          },
          "&.Mui-focused": {
            color: "#09090b",
          },
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          border: "1px solid #e4e4e7",
          boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.04)",
        },
      },
    },
  },
});

export default theme;
