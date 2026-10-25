"use client";

import { useState } from "react";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import AuthLayout from "@/components/auth/AuthLayout";
import AuthCard from "@/components/auth/AuthCard";
import AuthField from "@/components/auth/AuthField";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useForgotPasswordMutation, getApiErrorMessage } from "@/hooks/api/useAuth";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [feedback, setFeedback] = useState(null);

  const forgotPasswordMutation = useForgotPasswordMutation({
    onSuccess: (data) => {
      setIsSubmitted(true);
      setFeedback({
        type: "success",
        message:
          data?.message ||
          "If an account with that email exists, a password reset link has been sent.",
      });
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(error, "Failed to send reset instructions."),
      });
    },
  });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!email.trim()) {
      setFeedback({
        type: "error",
        message: "Please enter your registered email address.",
      });
      return;
    }

    forgotPasswordMutation.mutate({ email: email.trim() });
  };

  return (
    <AuthLayout>
      <AuthCard
        title="Forgot Password"
        subtitle="Enter the email associated with your account and we will send recovery instructions."
        footer={
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
            Remembered your password?{" "}
            <Link
              href="/login"
              style={{
                color: "#09090b",
                fontWeight: 600,
                textDecoration: "underline",
                textUnderlineOffset: 3,
              }}
            >
              Back to sign in
            </Link>
          </Typography>
        }
      >
        <AuthFeedback
          type={feedback?.type}
          message={feedback?.message}
          onClose={() => setFeedback(null)}
        />

        {isSubmitted ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography sx={{ fontSize: "0.875rem", color: "#09090b" }}>
              Please check your inbox. If the email is registered, you will find a link to reset your password.
            </Typography>
            <AuthButton
              type="button"
              variant="outlined"
              onClick={() => {
                setIsSubmitted(false);
                setEmail("");
              }}
            >
              Send to another email
            </AuthButton>
          </Box>
        ) : (
          <Box component="form" onSubmit={handleSubmit} noValidate>
            <AuthField
              id="forgot-email"
              name="email"
              type="email"
              label="Account Email"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                if (feedback) setFeedback(null);
              }}
              required
              autoComplete="email"
              autoFocus
            />

            <AuthButton
              type="submit"
              loading={forgotPasswordMutation.isPending}
              sx={{ mt: 1 }}
            >
              Send Reset Instructions
            </AuthButton>
          </Box>
        )}
      </AuthCard>
    </AuthLayout>
  );
}
