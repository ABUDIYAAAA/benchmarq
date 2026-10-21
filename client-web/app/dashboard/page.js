"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Paper from "@mui/material/Paper";
import CircularProgress from "@mui/material/CircularProgress";
import BrandLogo from "@/components/common/BrandLogo";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import OrgSwitcher from "@/components/org/OrgSwitcher";
import OrgZeroState from "@/components/org/OrgZeroState";
import OrgDashboardView from "@/components/org/OrgDashboardView";
import { useAuth } from "@/context/AuthContext";
import { useOrg } from "@/context/OrgContext";
import { useResendVerificationMutation } from "@/hooks/api/useAuth";

export default function AdminDashboardPage() {
  const router = useRouter();
  const { user, isLoading, isAuthenticated, logout, isLoggingOut } = useAuth();
  const { currentOrg, hasOrgs, isLoadingOrgs } = useOrg();

  const [bannerDismissed, setBannerDismissed] = useState(false);
  const [feedback, setFeedback] = useState(null);

  const resendMutation = useResendVerificationMutation({
    onSuccess: () => {
      setFeedback({
        type: "success",
        message: "Verification email dispatched. Please check your inbox.",
      });
    },
    onError: () => {
      setFeedback({
        type: "error",
        message: "Failed to dispatch verification email. Try again later.",
      });
    },
  });

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !user) {
    return (
      <Box
        sx={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: "#ffffff",
        }}
      >
        <CircularProgress size={24} sx={{ color: "#09090b" }} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        backgroundColor: "#fafafa",
      }}
    >
      {/* Top Header */}
      <Box
        component="header"
        sx={{
          px: { xs: 2.5, md: 5 },
          py: 2,
          backgroundColor: "#ffffff",
          borderBottom: "1px solid #e4e4e7",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
          <BrandLogo subtitle="Admin Console" />
          <Box sx={{ display: { xs: "none", sm: "block" } }}>
            <OrgSwitcher />
          </Box>
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", gap: 2.5 }}>
          <Box sx={{ textAlign: "right", display: { xs: "none", md: "block" } }}>
            <Typography sx={{ fontSize: "0.8125rem", fontWeight: 600, color: "#09090b" }}>
              {user.first_name} {user.last_name || ""}
            </Typography>
            <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
              {user.email}
            </Typography>
          </Box>
          <AuthButton
            variant="outlined"
            onClick={() => logout()}
            loading={isLoggingOut}
            fullWidth={false}
            sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
          >
            Sign Out
          </AuthButton>
        </Box>
      </Box>

      {/* Mobile Org Switcher Row */}
      <Box
        sx={{
          display: { xs: "block", sm: "none" },
          px: 2.5,
          py: 1.5,
          backgroundColor: "#ffffff",
          borderBottom: "1px solid #f4f4f5",
        }}
      >
        <OrgSwitcher />
      </Box>

      {/* Main Container */}
      <Container maxWidth="lg" sx={{ py: 4, flex: 1 }}>
        <AuthFeedback
          type={feedback?.type}
          message={feedback?.message}
          onClose={() => setFeedback(null)}
        />

        {/* Email Verification Banner */}
        {!user.is_email_verified && !bannerDismissed && (
          <Paper
            elevation={0}
            sx={{
              p: 2.5,
              mb: 3.5,
              backgroundColor: "#ffffff",
              border: "1px solid #09090b",
              borderRadius: 2,
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              gap: 2,
            }}
          >
            <Box sx={{ flex: 1 }}>
              <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
                Email Verification Required
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.25 }}>
                Your email ({user.email}) is not yet verified. Complete verification for full access.
              </Typography>
            </Box>

            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <AuthButton
                variant="outlined"
                fullWidth={false}
                loading={resendMutation.isPending}
                onClick={() => resendMutation.mutate()}
                sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
              >
                Resend Link
              </AuthButton>

              {/* Cross (dismiss) button */}
              <Box
                component="button"
                type="button"
                onClick={() => setBannerDismissed(true)}
                aria-label="Dismiss verification banner"
                sx={{
                  background: "transparent",
                  border: "none",
                  cursor: "pointer",
                  p: 0.5,
                  color: "#71717a",
                  fontSize: "1rem",
                  lineHeight: 1,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  "&:hover": { color: "#09090b" },
                }}
              >
                ✕
              </Box>
            </Box>
          </Paper>
        )}

        {/* Organization-Based Workspace Content */}
        {isLoadingOrgs ? (
          <Box sx={{ py: 8, display: "flex", justifyContent: "center" }}>
            <CircularProgress size={24} sx={{ color: "#09090b" }} />
          </Box>
        ) : !hasOrgs ? (
          <OrgZeroState />
        ) : currentOrg ? (
          <OrgDashboardView org={currentOrg} />
        ) : null}
      </Container>
    </Box>
  );
}
