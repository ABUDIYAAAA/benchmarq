"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import authApi from "@/lib/api/auth.api";

export const authKeys = {
  all: ["auth"],
  me: () => [...authKeys.all, "me"],
};

/**
 * Extracts human-readable error message from backend API response
 */
export function getApiErrorMessage(error, fallback = "An unexpected error occurred") {
  if (!error) return fallback;
  if (error.response?.data?.message) {
    return error.response.data.message;
  }
  if (error.response?.data?.errors && typeof error.response.data.errors === "object") {
    const errorList = Object.entries(error.response.data.errors).map(
      ([field, msg]) => `${field}: ${msg}`
    );
    if (errorList.length > 0) return errorList.join(" • ");
  }
  return error.message || fallback;
}

/**
 * Hook to retrieve current authenticated user session
 */
export function useCurrentUser(options = {}) {
  return useQuery({
    queryKey: authKeys.me(),
    queryFn: async () => {
      const response = await authApi.getMe();
      return response.data; // UserResponse object
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
    ...options,
  });
}

/**
 * Hook to authenticate user credentials
 */
export function useLoginMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (credentials) => authApi.login(credentials),
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(authKeys.me(), data.data?.user || null);
      queryClient.invalidateQueries({ queryKey: authKeys.me() });
      // Another account may have used this tab: refetch everything else
      // (organizations, exams...) for the new user. Reset, not remove, so
      // mounted queries keep working.
      queryClient.resetQueries({ predicate: (q) => q.queryKey[0] !== authKeys.all[0] });
      options.onSuccess?.(data, variables, context);
    },
  });
}

/**
 * Hook to register a new account
 */
export function useSignUpMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (payload) => authApi.signup(payload),
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(authKeys.me(), data.data?.user || null);
      queryClient.invalidateQueries({ queryKey: authKeys.me() });
      // Another account may have used this tab: refetch everything else
      // (organizations, exams...) for the new user. Reset, not remove, so
      // mounted queries keep working.
      queryClient.resetQueries({ predicate: (q) => q.queryKey[0] !== authKeys.all[0] });
      options.onSuccess?.(data, variables, context);
    },
  });
}

/**
 * Hook to log out the active session
 */
export function useLogoutMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: () => authApi.logout(),
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(authKeys.me(), null);
      queryClient.removeQueries({ queryKey: authKeys.all });
      options.onSuccess?.(data, variables, context);
    },
  });
}

/**
 * Hook to request password recovery email
 */
export function useForgotPasswordMutation(options = {}) {
  return useMutation({
    ...options,
    mutationFn: (payload) => authApi.forgotPassword(payload),
  });
}

/**
 * Hook to finalize password reset using security token
 */
export function useResetPasswordMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (payload) => authApi.resetPassword(payload),
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(authKeys.me(), null);
      queryClient.removeQueries({ queryKey: authKeys.all });
      options.onSuccess?.(data, variables, context);
    },
  });
}

/**
 * Hook to verify email with token
 */
export function useVerifyEmailMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (payload) => authApi.verifyEmail(payload),
    onSuccess: (data, variables, context) => {
      queryClient.invalidateQueries({ queryKey: authKeys.me() });
      options.onSuccess?.(data, variables, context);
    },
  });
}

/**
 * Hook to resend email verification token
 */
export function useResendVerificationMutation(options = {}) {
  return useMutation({
    ...options,
    mutationFn: () => authApi.resendVerification(),
  });
}

// Re-export AuthContext hook for consumer convenience
export { useAuth } from "@/context/AuthContext";

