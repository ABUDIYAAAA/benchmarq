import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";

export default function AuthButton({
  children,
  type = "submit",
  variant = "contained",
  loading = false,
  disabled = false,
  onClick,
  fullWidth = true,
  sx = {},
}) {
  const isContained = variant === "contained";

  return (
    <Button
      type={type}
      variant={variant}
      fullWidth={fullWidth}
      disabled={disabled || loading}
      onClick={onClick}
      sx={{
        py: 1.15,
        fontSize: "0.875rem",
        fontWeight: 500,
        letterSpacing: "-0.01em",
        position: "relative",
        ...sx,
      }}
    >
      {loading ? (
        <CircularProgress
          size={18}
          thickness={4.5}
          sx={{
            color: isContained ? "#ffffff" : "#09090b",
          }}
        />
      ) : (
        children
      )}
    </Button>
  );
}
