"use client";

import { Suspense, useState } from "react";
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
import { useResetPasswordMutation, getApiErrorMessage } from "@/hooks/api/useAuth";

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const tokenFromUrl = searchParams.get("token") || "";

  const [tokenInput, setTokenInput] = useState(tokenFromUrl);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isSuccess, setIsSuccess] = useState(false);
  const [feedback, setFeedback] = useState(null);

  const resetPasswordMutation = useResetPasswordMutation({
    onSuccess: () => {
      setIsSuccess(true);
      setFeedback({
        type: "success",
        message: "Your password has been successfully reset. You may now sign in.",
      });
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(
          error,
          "Invalid or expired password reset token. Please request a new link."
        ),
      });
    },
  });

  const handleSubmit = (e) => {
    e.preventDefault();

    const effectiveToken = tokenInput.trim();
    if (!effectiveToken) {
      setFeedback({
        type: "error",
        message: "Security reset token is required.",
      });
      return;
    }

    if (newPassword.length < 8) {
      setFeedback({
        type: "error",
        message: "Password must be at least 8 characters long.",
      });
      return;
    }

    if (newPassword !== confirmPassword) {
      setFeedback({
        type: "error",
        message: "Passwords do not match.",
      });
      return;
    }

    resetPasswordMutation.mutate({
      token: effectiveToken,
      new_password: newPassword,
    });
  };

  return (
    <AuthCard
      title="Reset Password"
      subtitle="Enter your verification token and specify a new secure password."
      footer={
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
          Return to{" "}
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
        </Typography>
      }
    >
      <AuthFeedback
        type={feedback?.type}
        message={feedback?.message}
        onClose={() => setFeedback(null)}
      />

      {isSuccess ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography sx={{ fontSize: "0.875rem", color: "#09090b" }}>
            Your credentials have been updated securely. Previous sessions have been invalidated.
          </Typography>
          <AuthButton
            type="button"
            onClick={() => router.push("/login")}
          >
            Proceed to Sign In
          </AuthButton>
        </Box>
      ) : (
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <AuthField
            id="reset-token"
            name="token"
            label="Reset Token"
            placeholder="Paste token or link code"
            value={tokenInput}
            onChange={(e) => {
              setTokenInput(e.target.value);
              if (feedback) setFeedback(null);
            }}
            required
            autoComplete="off"
            helperText={tokenFromUrl ? "Pre-populated from reset link" : ""}
          />

          <AuthField
            id="reset-new-password"
            name="newPassword"
            type="password"
            label="New Password"
            placeholder="Minimum 8 characters"
            value={newPassword}
            onChange={(e) => {
              setNewPassword(e.target.value);
              if (feedback) setFeedback(null);
            }}
            required
            autoComplete="new-password"
          />

          <AuthField
            id="reset-confirm-password"
            name="confirmPassword"
            type="password"
            label="Confirm New Password"
            placeholder="Repeat new password"
            value={confirmPassword}
            onChange={(e) => {
              setConfirmPassword(e.target.value);
              if (feedback) setFeedback(null);
            }}
            required
            autoComplete="new-password"
          />

          <AuthButton
            type="submit"
            loading={resetPasswordMutation.isPending}
            sx={{ mt: 1 }}
          >
            Update Password
          </AuthButton>
        </Box>
      )}
    </AuthCard>
  );
}

export default function ResetPasswordPage() {
  return (
    <AuthLayout>
      <Suspense
        fallback={
          <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
            <CircularProgress size={24} sx={{ color: "#09090b" }} />
          </Box>
        }
      >
        <ResetPasswordForm />
      </Suspense>
    </AuthLayout>
  );
}
