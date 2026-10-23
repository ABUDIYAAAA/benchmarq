"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import DashboardSidebar from "@/components/layout/DashboardSidebar";
import { useAuth } from "@/context/AuthContext";

export default function DashboardLayout({ children }) {
  const router = useRouter();
  const { user, isLoading, isAuthenticated } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !user) {
    return (
      <Box
        sx={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: "#ffffff",
        }}
      >
        <CircularProgress size={24} sx={{ color: "#09090b" }} />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", minHeight: "100vh", backgroundColor: "#fafafa" }}>
      {/* Fixed Sidebar */}
      <Box sx={{ display: { xs: "none", md: "block" } }}>
        <DashboardSidebar />
      </Box>

      {/* Main Page Area */}
      <Box
        component="main"
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          minWidth: 0,
          overflowX: "hidden",
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
