"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Divider from "@mui/material/Divider";
import ButtonBase from "@mui/material/ButtonBase";
import { useOrg } from "@/context/OrgContext";
import CreateOrgDialog from "./CreateOrgDialog";

export default function OrgSwitcher() {
  const { organizations, currentOrg, selectOrg } = useOrg();
  const [anchorEl, setAnchorEl] = useState(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const open = Boolean(anchorEl);

  const handleClick = (event) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleSelect = (org) => {
    selectOrg(org);
    handleClose();
  };

  const handleOpenCreate = () => {
    handleClose();
    setCreateDialogOpen(true);
  };

  if (!currentOrg) {
    return (
      <>
        <ButtonBase
          onClick={() => setCreateDialogOpen(true)}
          sx={{
            px: 1.5,
            py: 0.75,
            border: "1px dashed #a1a1aa",
            borderRadius: "6px",
            fontSize: "0.8125rem",
            fontWeight: 500,
            color: "#09090b",
            "&:hover": {
              backgroundColor: "#f4f4f5",
              borderColor: "#09090b",
            },
          }}
        >
          + Create Organization
        </ButtonBase>
        <CreateOrgDialog
          open={createDialogOpen}
          onClose={() => setCreateDialogOpen(false)}
        />
      </>
    );
  }

  return (
    <>
      <ButtonBase
        onClick={handleClick}
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1.25,
          px: 1.5,
          py: 0.75,
          borderRadius: "6px",
          border: "1px solid #e4e4e7",
          backgroundColor: "#ffffff",
          transition: "background-color 120ms ease-out, border-color 120ms ease-out",
          "&:hover": {
            backgroundColor: "#f4f4f5",
            borderColor: "#a1a1aa",
          },
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-start" }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography
              sx={{
                fontSize: "0.875rem",
                fontWeight: 600,
                color: "#09090b",
                maxWidth: 160,
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {currentOrg.name}
            </Typography>
            <Box
              sx={{
                fontSize: "0.625rem",
                fontWeight: 700,
                textTransform: "uppercase",
                px: 0.75,
                py: 0.1,
                borderRadius: "4px",
                backgroundColor: "#f4f4f5",
                color: "#71717a",
                border: "1px solid #e4e4e7",
              }}
            >
              {currentOrg.my_role || "member"}
            </Box>
          </Box>
        </Box>

        {/* Minimal Chevron SVG */}
        <Box
          component="span"
          sx={{
            display: "inline-flex",
            color: "#71717a",
            transform: open ? "rotate(180deg)" : "none",
            transition: "transform 140ms ease-out",
          }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </Box>
      </ButtonBase>

      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        slotProps={{
          paper: {
            elevation: 0,
            sx: {
              mt: 1,
              minWidth: 220,
              border: "1px solid #e4e4e7",
              borderRadius: "6px",
              boxShadow: "0 4px 12px rgba(0, 0, 0, 0.05)",
              p: 0.5,
            },
          },
        }}
      >
        <Typography
          sx={{
            px: 2,
            py: 1,
            fontSize: "0.6875rem",
            fontWeight: 700,
            color: "#71717a",
            textTransform: "uppercase",
            letterSpacing: "0.08em",
          }}
        >
          Organizations
        </Typography>

        {organizations.map((org) => {
          const isSelected = org.id === currentOrg.id;
          return (
            <MenuItem
              key={org.id}
              onClick={() => handleSelect(org)}
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                borderRadius: "4px",
                py: 1,
                px: 2,
                backgroundColor: isSelected ? "#f4f4f5" : "transparent",
                "&:hover": {
                  backgroundColor: "#f4f4f5",
                },
              }}
            >
              <Box>
                <Typography sx={{ fontSize: "0.875rem", fontWeight: isSelected ? 600 : 500, color: "#09090b" }}>
                  {org.name}
                </Typography>
                <Typography sx={{ fontSize: "0.6875rem", color: "#71717a" }}>
                  {org.my_role}
                </Typography>
              </Box>
              {isSelected && (
                <Typography sx={{ fontSize: "0.875rem", fontWeight: 700, color: "#09090b" }}>
                  ✓
                </Typography>
              )}
            </MenuItem>
          );
        })}

        <Divider sx={{ my: 0.75, borderColor: "#f4f4f5" }} />

        <MenuItem
          onClick={handleOpenCreate}
          sx={{
            borderRadius: "4px",
            py: 1,
            px: 2,
            fontSize: "0.8125rem",
            fontWeight: 500,
            color: "#09090b",
            "&:hover": {
              backgroundColor: "#f4f4f5",
            },
          }}
        >
          + New Organization
        </MenuItem>
      </Menu>

      <CreateOrgDialog
        open={createDialogOpen}
        onClose={() => setCreateDialogOpen(false)}
      />
    </>
  );
}
