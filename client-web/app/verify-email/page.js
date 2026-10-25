"use client";

import { Suspense, useState, useEffect, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import CircularProgress from "@mui/material/CircularProgress";
import AuthLayout from "@/components/auth/AuthLayout";
import AuthCard from "@/components/auth/AuthCard";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import {
  useVerifyEmailMutation,
  useResendVerificationMutation,
  getApiErrorMessage,
} from "@/hooks/api/useAuth";

function VerifyEmailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const tokenFromUrl = searchParams.get("token") || "";

  const [feedback, setFeedback] = useState(null);
  const [isVerified, setIsVerified] = useState(false);
  const hasAutoSubmitted = useRef(false);

  const verifyMutation = useVerifyEmailMutation({
    onSuccess: () => {
      setIsVerified(true);
      setFeedback({
        type: "success",
        message: "Email address verified successfully. Your account is fully activated.",
      });
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(
          error,
          "This verification link is invalid or has expired. Please request a new link."
        ),
      });
    },
  });

  const resendMutation = useResendVerificationMutation({
    onSuccess: () => {
      setFeedback({
        type: "success",
        message: "A new verification email has been sent. Please check your inbox.",
      });
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(
          error,
          "Unable to send verification email. Please ensure you are logged in or try again shortly."
        ),
      });
    },
  });

  // Automatically attempt verification once if token exists in query string
  useEffect(() => {
    if (tokenFromUrl && !hasAutoSubmitted.current) {
      hasAutoSubmitted.current = true;
      verifyMutation.mutate({ token: tokenFromUrl });
    }
  }, [tokenFromUrl, verifyMutation]);

  const handleResend = () => {
    setFeedback(null);
    resendMutation.mutate();
  };

  return (
    <AuthCard
      title={isVerified ? "Email Verified" : "Verify Email Address"}
      subtitle={
        isVerified
          ? "Your account has been successfully verified."
          : "Confirm your email address to access all features."
      }
      footer={
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
          Proceed to{" "}
          <Link
            href="/dashboard"
            style={{
              color: "#09090b",
              fontWeight: 600,
              textDecoration: "underline",
              textUnderlineOffset: 3,
            }}
          >
            Dashboard
          </Link>
          {" • "}
          <Link
            href="/login"
            style={{
              color: "#09090b",
              fontWeight: 600,
              textDecoration: "underline",
              textUnderlineOffset: 3,
            }}
          >
            Sign In
          </Link>
        </Typography>
      }
    >
      <AuthFeedback
        type={feedback?.type}
        message={feedback?.message}
        onClose={() => setFeedback(null)}
      />

      {isVerified ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box
            sx={{
              py: 2.5,
              px: 2,
              backgroundColor: "#fafafa",
              borderRadius: "8px",
              border: "1px solid #e4e4e7",
              textAlign: "center",
            }}
          >
            <Box
              sx={{
                width: 40,
                height: 40,
                borderRadius: "50%",
                backgroundColor: "#09090b",
                color: "#ffffff",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                margin: "0 auto 10px",
              }}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </Box>
            <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
              Account Activated
            </Typography>
            <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.25 }}>
              Your account is now active.
            </Typography>
          </Box>

          <AuthButton
            type="button"
            onClick={() => router.push("/dashboard")}
          >
            Continue to Dashboard
          </AuthButton>
        </Box>
      ) : verifyMutation.isPending ? (
        <Box
          sx={{
            py: 5,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 2,
          }}
        >
          <CircularProgress size={26} sx={{ color: "#09090b" }} />
          <Typography sx={{ fontSize: "0.875rem", color: "#71717a" }}>
            Verifying your email link...
          </Typography>
        </Box>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          <Box
            sx={{
              p: 2.5,
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
                backgroundColor: "#f4f4f5",
                color: "#71717a",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                margin: "0 auto 12px",
              }}
            >
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                <polyline points="22,6 12,13 2,6" />
              </svg>
            </Box>
            <Typography sx={{ fontSize: "0.9375rem", fontWeight: 600, color: "#09090b" }}>
              Check Your Inbox
            </Typography>
            <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.75, lineHeight: 1.5 }}>
              Click the link inside the verification email sent to your address to activate your account.
            </Typography>
          </Box>

          <AuthButton
            type="button"
            variant="contained"
            loading={resendMutation.isPending}
            onClick={handleResend}
          >
            Resend Verification Link
          </AuthButton>

          <AuthButton
            type="button"
            variant="outlined"
            onClick={() => router.push("/dashboard")}
          >
            Go to Dashboard
          </AuthButton>
        </Box>
      )}
    </AuthCard>
  );
}

export default function VerifyEmailPage() {
  return (
    <AuthLayout>
      <Suspense
        fallback={
          <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
            <CircularProgress size={24} sx={{ color: "#09090b" }} />
          </Box>
        }
      >
        <VerifyEmailContent />
      </Suspense>
    </AuthLayout>
  );
}
