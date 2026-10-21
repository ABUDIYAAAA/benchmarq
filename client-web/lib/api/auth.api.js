import apiClient from "./axios";

/**
 * Auth API Service
 * All methods communicate via cookie-based sessions.
 */
export const authApi = {
  /**
   * Register a new administrator user
   * @param {{ email: string, password: string, first_name: string, last_name?: string }} data
   */
  async signup(data) {
    const res = await apiClient.post("/auth/signup", data);
    return res.data;
  },

  /**
   * Authenticate administrator
   * @param {{ email: string, password: string }} data
   */
  async login(data) {
    const res = await apiClient.post("/auth/login", data);
    return res.data;
  },

  /**
   * Terminate active session and clear cookies
   */
  async logout() {
    const res = await apiClient.post("/auth/logout", {});
    return res.data;
  },

  /**
   * Explicitly rotate refresh token
   */
  async refreshToken() {
    const res = await apiClient.post("/auth/refresh", {});
    return res.data;
  },

  /**
   * Fetch current authenticated user
   */
  async getMe() {
    const res = await apiClient.get("/auth/me");
    return res.data;
  },

  /**
   * Request password reset instructions
   * @param {{ email: string }} data
   */
  async forgotPassword(data) {
    const res = await apiClient.post("/auth/forgot-password", data);
    return res.data;
  },

  /**
   * Reset account password using token
   * @param {{ token: string, new_password: string }} data
   */
  async resetPassword(data) {
    const res = await apiClient.post("/auth/reset-password", data);
    return res.data;
  },

  /**
   * Verify email using token
   * @param {{ token: string }} data
   */
  async verifyEmail(data) {
    const res = await apiClient.post("/auth/verify-email", data);
    return res.data;
  },

  /**
   * Resend email verification link to current authenticated user
   */
  async resendVerification() {
    const res = await apiClient.post("/auth/resend-verification", {});
    return res.data;
  },
};

export default authApi;
