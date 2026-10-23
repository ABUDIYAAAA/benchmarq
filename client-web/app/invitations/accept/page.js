"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import CircularProgress from "@mui/material/CircularProgress";
import ButtonBase from "@mui/material/ButtonBase";
import AuthLayout from "@/components/auth/AuthLayout";
import AuthCard from "@/components/auth/AuthCard";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useAuth } from "@/context/AuthContext";
import { useOrg } from "@/context/OrgContext";
import { useToast } from "@/context/ToastContext";
import { useQueryClient } from "@tanstack/react-query";
import { orgKeys } from "@/hooks/api/useOrgs";
import orgsApi from "@/lib/api/orgs.api";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

function AcceptInvitationContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const { user, isAuthenticated, isLoading: isLoadingAuth, logout } = useAuth();
  const { selectOrg } = useOrg();
  const { showSuccess } = useToast();
  const queryClient = useQueryClient();

  const [isAccepting, setIsAccepting] = useState(false);
  const [acceptedOrg, setAcceptedOrg] = useState(null);
  const [errorFeedback, setErrorFeedback] = useState(null);

  const handleAccept = async () => {
    if (!token) {
      setErrorFeedback("Invalid or missing invitation token.");
      return;
    }

    setIsAccepting(true);
    setErrorFeedback(null);

    try {
      const res = await orgsApi.acceptInvitation(token);
      const joinedOrg = res?.data || res;
      setAcceptedOrg(joinedOrg);

      // Invalidate organizations cache and switch to newly joined org
      if (joinedOrg?.id) {
        queryClient.invalidateQueries({ queryKey: orgKeys.mine() });
        selectOrg(joinedOrg.id);
      }

      showSuccess(`Welcome to ${joinedOrg?.name || "the organization"}!`);
    } catch (err) {
      const msg = getApiErrorMessage(
        err,
        "Failed to accept invitation. The link may have expired or is invalid."
      );
      setErrorFeedback(msg);
    } finally {
      setIsAccepting(false);
    }
  };

  const handleSwitchAccount = async () => {
    try {
      await logout();
    } catch {
      // Ignore logout errors
    }
    const currentUrl = `/invitations/accept?token=${encodeURIComponent(token)}`;
    router.push(`/login?redirect=${encodeURIComponent(currentUrl)}`);
  };

  if (isLoadingAuth) {
    return (
      <Box sx={{ py: 6, display: "flex", justifyContent: "center" }}>
        <CircularProgress size={28} sx={{ color: "#09090b" }} />
      </Box>
    );
  }

  // Case 1: No Token in URL
  if (!token) {
    return (
      <AuthCard
        title="Invalid Invitation Link"
        subtitle="The invitation link appears to be malformed or missing a security token."
      >
        <AuthFeedback
          type="error"
          message="No invitation token was found in the URL. Please verify the link in your email."
        />
        <Box sx={{ mt: 3 }}>
          <AuthButton variant="contained" onClick={() => router.push("/dashboard")}>
            Go to Dashboard
          </AuthButton>
        </Box>
      </AuthCard>
    );
  }

  // Case 2: User is NOT logged in
  if (!isAuthenticated) {
    const returnUrl = `/invitations/accept?token=${encodeURIComponent(token)}`;
    return (
      <AuthCard
        title="Organization Invitation"
        subtitle="You have been invited to join a workspace on Benchmarq."
        footer={
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
            Need help? Contact your workspace administrator for a new invite.
          </Typography>
        }
      >
        <Box
          sx={{
            p: 2.5,
            mb: 3,
            backgroundColor: "#f4f4f5",
            borderRadius: "6px",
            border: "1px solid #e4e4e7",
            textAlign: "left",
          }}
        >
          <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b", mb: 0.5 }}>
            Sign In Required
          </Typography>
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", lineHeight: 1.5 }}>
            To accept this invitation and access team assessments and resources, please sign in with the email address where you received the invite.
          </Typography>
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <AuthButton
            variant="contained"
            onClick={() => router.push(`/login?redirect=${encodeURIComponent(returnUrl)}`)}
          >
            Sign In to Accept
          </AuthButton>

          <AuthButton
            variant="outlined"
            onClick={() => router.push(`/signup?redirect=${encodeURIComponent(returnUrl)}`)}
          >
            Create an Account
          </AuthButton>
        </Box>
      </AuthCard>
    );
  }

  // Case 3: Invitation Accepted Successfully
  if (acceptedOrg) {
    return (
      <AuthCard
        title="Invitation Accepted"
        subtitle={`You are now a member of ${acceptedOrg.name}.`}
      >
        <Box
          sx={{
            py: 3,
            px: 2.5,
            mb: 3,
            backgroundColor: "#fafafa",
            borderRadius: "8px",
            border: "1px solid #e4e4e7",
            textAlign: "center",
          }}
        >
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: "50%",
              backgroundColor: "#09090b",
              color: "#ffffff",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              margin: "0 auto 12px",
            }}
          >
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </Box>
          <Typography sx={{ fontSize: "1.125rem", fontWeight: 600, color: "#09090b" }}>
            {acceptedOrg.name}
          </Typography>
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
            Role: <strong style={{ color: "#09090b", textTransform: "capitalize" }}>{acceptedOrg.my_role || "Member"}</strong>
          </Typography>
        </Box>

        <AuthButton variant="contained" onClick={() => router.push("/dashboard")}>
          Go to Dashboard
        </AuthButton>
      </AuthCard>
    );
  }

  // Case 4: Logged in & Ready to Accept
  const isWrongEmail = errorFeedback && errorFeedback.toLowerCase().includes("different email");

  return (
    <AuthCard
      title="Accept Invitation"
      subtitle="Confirm your membership to collaborate on Benchmarq."
    >
      <AuthFeedback
        type="error"
        message={errorFeedback}
        onClose={() => setErrorFeedback(null)}
      />

      {/* Current User Pill */}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          p: 1.5,
          mb: 3,
          borderRadius: "6px",
          border: "1px solid #e4e4e7",
          backgroundColor: "#fafafa",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
          <Box
            sx={{
              width: 28,
              height: 28,
              borderRadius: "50%",
              backgroundColor: "#09090b",
              color: "#ffffff",
              fontSize: "0.75rem",
              fontWeight: 600,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            {user?.first_name?.[0]?.toUpperCase() || "U"}
          </Box>
          <Box>
            <Typography sx={{ fontSize: "0.8125rem", fontWeight: 500, color: "#09090b", lineHeight: 1.2 }}>
              {user?.first_name} {user?.last_name || ""}
            </Typography>
            <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
              {user?.email}
            </Typography>
          </Box>
        </Box>

        <ButtonBase
          onClick={handleSwitchAccount}
          sx={{
            fontSize: "0.75rem",
            color: "#71717a",
            textDecoration: "underline",
            textUnderlineOffset: 2,
            "&:hover": { color: "#09090b" },
          }}
        >
          Switch
        </ButtonBase>
      </Box>

      {isWrongEmail && (
        <Box
          sx={{
            p: 2,
            mb: 2.5,
            borderRadius: "6px",
            backgroundColor: "#fef2f2",
            border: "1px solid #fecaca",
          }}
        >
          <Typography sx={{ fontSize: "0.8125rem", color: "#991b1b", lineHeight: 1.4 }}>
            This invitation was addressed to a different email. Please switch to the invited account to accept.
          </Typography>
        </Box>
      )}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        <AuthButton
          variant="contained"
          loading={isAccepting}
          onClick={handleAccept}
        >
          Accept Invitation
        </AuthButton>

        <AuthButton
          variant="outlined"
          disabled={isAccepting}
          onClick={() => router.push("/dashboard")}
        >
          Decline & Go to Dashboard
        </AuthButton>
      </Box>
    </AuthCard>
  );
}

export default function AcceptInvitationPage() {
  return (
    <AuthLayout>
      <Suspense
        fallback={
          <Box sx={{ py: 6, display: "flex", justifyContent: "center" }}>
            <CircularProgress size={28} sx={{ color: "#09090b" }} />
          </Box>
        }
      >
        <AcceptInvitationContent />
      </Suspense>
    </AuthLayout>
  );
}
