import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import BrandLogo from "@/components/common/BrandLogo";

export default function AuthLayout({ children }) {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        backgroundColor: "#fafafa",
        borderTop: "2px solid #09090b",
      }}
    >
      {/* Top Header */}
      <Box
        component="header"
        sx={{
          py: 3,
          px: { xs: 3, sm: 5 },
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          borderBottom: "1px solid #f4f4f5",
          backgroundColor: "#ffffff",
        }}
      >
        <BrandLogo />
      </Box>

      {/* Main Form Center Area */}
      <Container
        maxWidth="sm"
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          alignItems: "center",
          py: { xs: 4, sm: 6 },
        }}
      >
        {children}
      </Container>

      {/* Bottom Minimal Footer */}
      <Box
        component="footer"
        sx={{
          py: 2.5,
          px: 3,
          borderTop: "1px solid #f4f4f5",
          backgroundColor: "#ffffff",
          textAlign: "center",
        }}
      >
        <Typography sx={{ fontSize: "0.75rem", color: "#a1a1aa" }}>
          © {new Date().getFullYear()} Benchmarq Inc. All rights reserved.
        </Typography>
      </Box>
    </Box>
  );
}
