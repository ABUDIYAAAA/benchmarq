"use client";

import { Suspense, useState, useEffect, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import CircularProgress from "@mui/material/CircularProgress";
import AuthLayout from "@/components/auth/AuthLayout";
import AuthCard from "@/components/auth/AuthCard";
import AuthField from "@/components/auth/AuthField";
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

  const [tokenInput, setTokenInput] = useState(tokenFromUrl);
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
          "Invalid or expired verification link. Please request a new one."
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
          "Unable to send verification email. Ensure you are signed in or try again shortly."
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

  const handleSubmit = (e) => {
    e.preventDefault();
    const effectiveToken = tokenInput.trim();
    if (!effectiveToken) {
      setFeedback({
        type: "error",
        message: "Please enter the verification token.",
      });
      return;
    }

    verifyMutation.mutate({ token: effectiveToken });
  };

  const handleResend = () => {
    setFeedback(null);
    resendMutation.mutate();
  };

  return (
    <AuthCard
      title="Verify Email Address"
      subtitle="Confirm your administrator email address to maintain security compliance."
      footer={
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
          Proceed to{" "}
          <Link
            href="/login"
            style={{
              color: "#09090b",
              fontWeight: 600,
              textDecoration: "underline",
              textUnderlineOffset: 3,
            }}
          >
            Sign in
          </Link>
          {" • "}
          <Link
            href="/dashboard"
            style={{
              color: "#09090b",
              fontWeight: 600,
              textDecoration: "underline",
              textUnderlineOffset: 3,
            }}
          >
            Console
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
          <Typography sx={{ fontSize: "0.875rem", color: "#09090b" }}>
            Your email has been confirmed. You now have unhindered administrator privileges.
          </Typography>
          <AuthButton
            type="button"
            onClick={() => router.push("/dashboard")}
          >
            Go to Admin Dashboard
          </AuthButton>
        </Box>
      ) : verifyMutation.isPending ? (
        <Box
          sx={{
            py: 4,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 2,
          }}
        >
          <CircularProgress size={24} sx={{ color: "#09090b" }} />
          <Typography sx={{ fontSize: "0.875rem", color: "#71717a" }}>
            Verifying security token...
          </Typography>
        </Box>
      ) : (
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <AuthField
            id="verify-token"
            name="token"
            label="Verification Token"
            placeholder="Paste verification token"
            value={tokenInput}
            onChange={(e) => {
              setTokenInput(e.target.value);
              if (feedback) setFeedback(null);
            }}
            required
            autoComplete="off"
            helperText="Check your registered inbox for the verification token."
          />

          <AuthButton
            type="submit"
            loading={verifyMutation.isPending}
            sx={{ mt: 1 }}
          >
            Verify Email
          </AuthButton>

          <Box sx={{ mt: 2.5, textAlign: "center" }}>
            <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mb: 1 }}>
              Didn&apos;t receive the email?
            </Typography>
            <AuthButton
              type="button"
              variant="outlined"
              loading={resendMutation.isPending}
              onClick={handleResend}
            >
              Resend Verification Link
            </AuthButton>
          </Box>
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
