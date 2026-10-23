"use client";

import { Suspense, useState, useEffect } from "react";
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
import { useLoginMutation, getApiErrorMessage } from "@/hooks/api/useAuth";

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const isRegisteredParam = searchParams.get("registered") === "true";
  const emailParam = searchParams.get("email") || "";
  const redirectParam = searchParams.get("redirect") || "";

  const [formData, setFormData] = useState({
    email: emailParam,
    password: "",
  });

  const [feedback, setFeedback] = useState(() => {
    if (isRegisteredParam) {
      return {
        type: "success",
        message: "Account created successfully. Please enter your password to sign in.",
      };
    }
    return null;
  });

  const loginMutation = useLoginMutation({
    onSuccess: () => {
      router.push(redirectParam || "/dashboard");
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(error, "Invalid email or password. Please verify your credentials."),
      });
    },
  });

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
    if (feedback) setFeedback(null);
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!formData.email.trim() || !formData.password) {
      setFeedback({
        type: "error",
        message: "Please provide both email and password.",
      });
      return;
    }

    loginMutation.mutate({
      email: formData.email.trim(),
      password: formData.password,
    });
  };

  return (
    <AuthCard
      title="Admin Sign In"
      subtitle="Authenticate with your credentials to manage exams, organizations, and candidates."
      footer={
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
          Need an administrator account?{" "}
          <Link
            href={redirectParam ? `/signup?redirect=${encodeURIComponent(redirectParam)}` : "/signup"}
            style={{
              color: "#09090b",
              fontWeight: 600,
              textDecoration: "underline",
              textUnderlineOffset: 3,
            }}
          >
            Sign up
          </Link>
        </Typography>
      }
    >
      <AuthFeedback
        type={feedback?.type}
        message={feedback?.message}
        onClose={() => setFeedback(null)}
      />

      <Box component="form" onSubmit={handleSubmit} noValidate>
        <AuthField
          id="login-email"
          name="email"
          type="email"
          label="Email Address"
          placeholder="admin@benchmarq.io"
          value={formData.email}
          onChange={handleChange}
          required
          autoComplete="email"
          autoFocus={!formData.email}
        />

        <Box sx={{ position: "relative" }}>
          <Box
            sx={{
              display: "flex",
              justifyContent: "flex-end",
              position: "absolute",
              top: 0,
              right: 0,
              zIndex: 1,
            }}
          >
            <Link
              href="/forgot-password"
              style={{
                fontSize: "0.75rem",
                color: "#71717a",
                textDecoration: "none",
              }}
            >
              Forgot password?
            </Link>
          </Box>
          <AuthField
            id="login-password"
            name="password"
            type="password"
            label="Password"
            placeholder="••••••••"
            value={formData.password}
            onChange={handleChange}
            required
            autoComplete="current-password"
            autoFocus={Boolean(formData.email)}
          />
        </Box>

        <AuthButton
          type="submit"
          loading={loginMutation.isPending}
          sx={{ mt: 1 }}
        >
          Sign In
        </AuthButton>
      </Box>
    </AuthCard>
  );
}

export default function LoginPage() {
  return (
    <AuthLayout>
      <Suspense
        fallback={
          <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
            <CircularProgress size={24} sx={{ color: "#09090b" }} />
          </Box>
        }
      >
        <LoginForm />
      </Suspense>
    </AuthLayout>
  );
}
