/** @type {import('next').NextConfig} */
const nextConfig = {
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async rewrites() {
    return [
      { source: "/auth/login", destination: "/login" },
      { source: "/auth/signup", destination: "/signup" },
      { source: "/auth/forgot-password", destination: "/forgot-password" },
      { source: "/auth/reset-password", destination: "/reset-password" },
      { source: "/auth/verify-email", destination: "/verify-email" },
    ];
  },
};

export default nextConfig;
