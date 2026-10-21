import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Link from "next/link";

export default function BrandLogo({ subtitle = "" }) {
  return (
    <Link href="/" style={{ textDecoration: "none", color: "inherit" }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
        {/* Geometric minimalist logo glyph */}
        <Box
          sx={{
            width: 28,
            height: 28,
            backgroundColor: "#09090b",
            color: "#ffffff",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: "4px",
            fontSize: "0.875rem",
            fontWeight: 700,
            letterSpacing: "0.05em",
          }}
        >
          B
        </Box>
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <Typography
            sx={{
              fontWeight: 700,
              fontSize: "0.9375rem",
              letterSpacing: "-0.01em",
              color: "#09090b",
              lineHeight: 1.1,
            }}
          >
            BENCHMARQ
          </Typography>
          {subtitle && (
            <Typography
              sx={{
                fontSize: "0.6875rem",
                color: "#71717a",
                letterSpacing: "0.06em",
                textTransform: "uppercase",
                fontWeight: 500,
                mt: 0.25,
              }}
            >
              {subtitle}
            </Typography>
          )}
        </Box>
      </Box>
    </Link>
  );
}
