"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import BrandLogo from "@/components/common/BrandLogo";
import OrgSwitcher from "@/components/org/OrgSwitcher";
import { useAuth } from "@/context/AuthContext";
import { useOrg } from "@/context/OrgContext";

const NAV_ITEMS = [
  {
    label: "Overview",
    href: "/dashboard",
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <rect width="7" height="9" x="3" y="3" rx="1" />
        <rect width="7" height="5" x="14" y="3" rx="1" />
        <rect width="7" height="9" x="14" y="12" rx="1" />
        <rect width="7" height="5" x="3" y="16" rx="1" />
      </svg>
    ),
  },
  {
    label: "Users & Team",
    href: "/dashboard/users",
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
        <path d="M16 3.13a4 4 0 0 1 0 7.75" />
      </svg>
    ),
  },
];

export default function DashboardSidebar() {
  const pathname = usePathname();
  const { user, logout, isLoggingOut } = useAuth();
  const { currentOrg, hasOrgs } = useOrg();

  return (
    <Box
      component="aside"
      sx={{
        width: 256,
        flexShrink: 0,
        backgroundColor: "#ffffff",
        borderRight: "1px solid #e4e4e7",
        display: "flex",
        flexDirection: "column",
        minHeight: "100vh",
      }}
    >
      {/* Top Header: Brand Logo */}
      <Box sx={{ p: 2.5, borderBottom: "1px solid #f4f4f5" }}>
        <BrandLogo subtitle="Admin Console" />
      </Box>

      {/* Workspace Switcher */}
      <Box sx={{ p: 2, borderBottom: "1px solid #f4f4f5" }}>
        <Typography
          sx={{
            fontSize: "0.6875rem",
            fontWeight: 700,
            color: "#71717a",
            textTransform: "uppercase",
            letterSpacing: "0.08em",
            mb: 1,
            px: 0.5,
          }}
        >
          Workspace
        </Typography>
        <OrgSwitcher />
      </Box>

      {/* Navigation Items */}
      <Box component="nav" sx={{ p: 2, flex: 1, display: "flex", flexDirection: "column", gap: 0.5 }}>
        {hasOrgs && (
          <>
            <Typography
              sx={{
                fontSize: "0.6875rem",
                fontWeight: 700,
                color: "#71717a",
                textTransform: "uppercase",
                letterSpacing: "0.08em",
                mb: 1,
                mt: 1,
                px: 0.5,
              }}
            >
              Management
            </Typography>

            {NAV_ITEMS.map((item) => {
              const isActive =
                item.href === "/dashboard"
                  ? pathname === "/dashboard"
                  : pathname.startsWith(item.href);

              return (
                <Link
                  key={item.href}
                  href={item.href}
                  style={{ textDecoration: "none", color: "inherit" }}
                >
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1.5,
                      px: 1.5,
                      py: 1,
                      borderRadius: "6px",
                      fontSize: "0.875rem",
                      fontWeight: isActive ? 600 : 500,
                      color: isActive ? "#09090b" : "#71717a",
                      backgroundColor: isActive ? "#f4f4f5" : "transparent",
                      transition: "background-color 120ms ease-out, color 120ms ease-out",
                      "&:hover": {
                        backgroundColor: "#f4f4f5",
                        color: "#09090b",
                      },
                    }}
                  >
                    <Box
                      component="span"
                      sx={{
                        color: isActive ? "#09090b" : "#a1a1aa",
                        display: "flex",
                        alignItems: "center",
                      }}
                    >
                      {item.icon}
                    </Box>
                    <Typography sx={{ fontSize: "inherit", fontWeight: "inherit", color: "inherit" }}>
                      {item.label}
                    </Typography>
                  </Box>
                </Link>
              );
            })}
          </>
        )}
      </Box>

      {/* Bottom Profile and Sign Out */}
      <Box sx={{ p: 2, borderTop: "1px solid #e4e4e7", backgroundColor: "#fafafa" }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1.5 }}>
          <Box sx={{ overflow: "hidden" }}>
            <Typography
              sx={{
                fontSize: "0.8125rem",
                fontWeight: 600,
                color: "#09090b",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {user?.first_name} {user?.last_name || ""}
            </Typography>
            <Typography
              sx={{
                fontSize: "0.75rem",
                color: "#71717a",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {user?.email}
            </Typography>
          </Box>
        </Box>

        <Box
          component="button"
          type="button"
          onClick={() => logout()}
          disabled={isLoggingOut}
          sx={{
            width: "100%",
            textAlign: "center",
            py: 0.85,
            px: 1.5,
            fontSize: "0.75rem",
            fontWeight: 500,
            color: "#09090b",
            backgroundColor: "#ffffff",
            border: "1px solid #e4e4e7",
            borderRadius: "6px",
            cursor: "pointer",
            transition: "background-color 120ms ease-out",
            "&:hover": {
              backgroundColor: "#f4f4f5",
            },
            "&:disabled": {
              color: "#a1a1aa",
              cursor: "not-allowed",
            },
          }}
        >
          {isLoggingOut ? "Signing Out..." : "Sign Out"}
        </Box>
      </Box>
    </Box>
  );
}
