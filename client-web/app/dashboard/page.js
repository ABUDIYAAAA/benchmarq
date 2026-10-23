"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Paper from "@mui/material/Paper";
import CircularProgress from "@mui/material/CircularProgress";
import AuthButton from "@/components/auth/AuthButton";
import BrandLogo from "@/components/common/BrandLogo";
import OrgSwitcher from "@/components/org/OrgSwitcher";
import OrgZeroState from "@/components/org/OrgZeroState";
import OrgDashboardView from "@/components/org/OrgDashboardView";
import { useAuth } from "@/context/AuthContext";
import { useOrg } from "@/context/OrgContext";
import { useResendVerificationMutation, getApiErrorMessage } from "@/hooks/api/useAuth";
import { isStaff } from "@/lib/permissions";

export default function AdminDashboardPage() {
  const { user } = useAuth();
  const { currentOrg, hasOrgs, isLoadingOrgs } = useOrg();
  const router = useRouter();
  const candidateOnly = Boolean(currentOrg) && !isStaff(currentOrg.my_role);

  // Candidates have nothing to manage; their home is the exams they can take.
  useEffect(() => {
    if (candidateOnly) router.replace("/dashboard/my-exams");
  }, [candidateOnly, router]);

  const [bannerDismissed, setBannerDismissed] = useState(false);
  const [resendStatus, setResendStatus] = useState("idle"); // "idle" | "sent" | "error"
  const [resendError, setResendError] = useState("");

  const resendMutation = useResendVerificationMutation({
    onSuccess: () => {
      setResendStatus("sent");
    },
    onError: (err) => {
      setResendStatus("error");
      setResendError(
        getApiErrorMessage(err, "Failed to dispatch verification email. Try again shortly.")
      );
    },
  });

  return (
    <Box sx={{ width: "100%", minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      {/* Mobile-only header */}
      <Box
        sx={{
          display: { xs: "flex", md: "none" },
          px: 2.5,
          py: 1.75,
          backgroundColor: "#ffffff",
          borderBottom: "1px solid #e4e4e7",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <BrandLogo subtitle="Admin Console" />
        <OrgSwitcher />
      </Box>

      {/* Main Container */}
      <Container maxWidth="lg" sx={{ py: 4, px: { xs: 2.5, sm: 4 }, flex: 1 }}>
        {/* Email Verification Banner */}
        {user && !user.is_email_verified && !bannerDismissed && (
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
              {resendStatus === "sent" ? (
                <>
                  <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
                    Verification Email Dispatched
                  </Typography>
                  <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.25 }}>
                    A fresh verification link has been sent to {user.email}. Check your inbox and spam folder.
                  </Typography>
                </>
              ) : (
                <>
                  <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
                    Email Verification Required
                  </Typography>
                  <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.25 }}>
                    Your email ({user.email}) is not yet verified. Complete verification for full access.
                  </Typography>
                  {resendStatus === "error" && (
                    <Typography sx={{ fontSize: "0.75rem", color: "#09090b", fontWeight: 500, mt: 0.5 }}>
                      {resendError}
                    </Typography>
                  )}
                </>
              )}
            </Box>

            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              {resendStatus !== "sent" && (
                <AuthButton
                  variant="outlined"
                  fullWidth={false}
                  loading={resendMutation.isPending}
                  onClick={() => resendMutation.mutate()}
                  sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
                >
                  Resend Link
                </AuthButton>
              )}

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

        {/* Workspace Content */}
        {isLoadingOrgs ? (
          <Box sx={{ py: 8, display: "flex", justifyContent: "center" }}>
            <CircularProgress size={24} sx={{ color: "#09090b" }} />
          </Box>
        ) : !hasOrgs ? (
          <OrgZeroState />
        ) : currentOrg && !candidateOnly ? (
          <OrgDashboardView org={currentOrg} />
        ) : null}
      </Container>
    </Box>
  );
}
