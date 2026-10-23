import axios from "axios";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8000/api/v1";

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true, // Required: ensures httpOnly session cookies are transmitted
  headers: {
    "Content-Type": "application/json",
  },
});

export const SESSION_EXPIRED_EVENT = "benchmarq:session-expired";

// Refresh token concurrency management. Concurrent 401s share one refresh;
// separate tabs may still refresh at the same moment, which the server
// tolerates within its reuse window.
let isRefreshing = false;
let failedQueue = [];

const processQueue = (error) => {
  failedQueue.forEach((promise) => {
    if (error) {
      promise.reject(error);
    } else {
      promise.resolve();
    }
  });
  failedQueue = [];
};

// Response interceptor to seamlessly rotate tokens via cookies on 401
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Do not attempt refresh on auth endpoints (login, signup, logout, refresh itself)
    const isAuthRoute = ["/auth/login", "/auth/signup", "/auth/logout", "/auth/refresh"].some((path) =>
      originalRequest?.url?.includes(path)
    );

    if (error.response?.status === 401 && !originalRequest?._retry && !isAuthRoute) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then(() => apiClient(originalRequest))
          .catch((err) => Promise.reject(err));
      }

      originalRequest._retry = true;
      isRefreshing = true;

      try {
        // Calls POST /api/v1/auth/refresh with withCredentials: true.
        // The backend reads the refresh_token cookie and writes rotated cookies.
        await apiClient.post("/auth/refresh", {});
        processQueue(null);
        return apiClient(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError);
        // The session is gone (expired, revoked or replayed): let the app
        // clear its state and send the user to sign in.
        if (typeof window !== "undefined") {
          window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
        }
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);

export default apiClient;
