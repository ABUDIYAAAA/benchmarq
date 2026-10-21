import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";

export default function AuthFeedback({ type = "error", message, onClose }) {
  if (!message) return null;

  const isError = type === "error";

  return (
    <Box
      role="alert"
      sx={{
        width: "100%",
        p: 1.5,
        mb: 2.5,
        borderRadius: "6px",
        border: "1px solid",
        borderColor: isError ? "#09090b" : "#e4e4e7",
        backgroundColor: isError ? "#fafafa" : "#ffffff",
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "space-between",
        gap: 1.5,
      }}
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
        <Typography
          sx={{
            fontSize: "0.6875rem",
            fontWeight: 700,
            textTransform: "uppercase",
            letterSpacing: "0.08em",
            color: isError ? "#09090b" : "#71717a",
          }}
        >
          {isError ? "Error" : "Notification"}
        </Typography>
        <Typography
          sx={{
            fontSize: "0.8125rem",
            color: "#09090b",
            lineHeight: 1.45,
            wordBreak: "break-word",
          }}
        >
          {message}
        </Typography>
      </Box>

      {onClose && (
        <Box
          component="button"
          type="button"
          onClick={onClose}
          sx={{
            background: "transparent",
            border: "none",
            p: 0.5,
            cursor: "pointer",
            color: "#71717a",
            fontSize: "0.875rem",
            lineHeight: 1,
            "&:hover": {
              color: "#09090b",
            },
          }}
          aria-label="Dismiss message"
        >
          ✕
        </Box>
      )}
    </Box>
  );
}
