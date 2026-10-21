import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import Divider from "@mui/material/Divider";

export default function AuthCard({
  title,
  subtitle,
  children,
  footer,
}) {
  return (
    <Paper
      elevation={0}
      sx={{
        width: "100%",
        maxWidth: 420,
        borderRadius: 2,
        backgroundColor: "#ffffff",
        border: "1px solid #e4e4e7",
        p: { xs: 3, sm: 4 },
      }}
    >
      <Box sx={{ mb: 3 }}>
        {title && (
          <Typography
            component="h1"
            sx={{
              fontSize: "1.25rem",
              fontWeight: 600,
              letterSpacing: "-0.02em",
              color: "#09090b",
              lineHeight: 1.25,
            }}
          >
            {title}
          </Typography>
        )}
        {subtitle && (
          <Typography
            sx={{
              mt: 0.75,
              fontSize: "0.8125rem",
              color: "#71717a",
              lineHeight: 1.45,
            }}
          >
            {subtitle}
          </Typography>
        )}
      </Box>

      <Box component="div">{children}</Box>

      {footer && (
        <>
          <Divider sx={{ my: 3, borderColor: "#f4f4f5" }} />
          <Box
            sx={{
              display: "flex",
              justifyContent: "center",
              fontSize: "0.8125rem",
              color: "#71717a",
            }}
          >
            {footer}
          </Box>
        </>
      )}
    </Paper>
  );
}
