"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";

export const dialogPaper = {
  paper: { elevation: 0, sx: { border: "1px solid #e4e4e7", borderRadius: 2 } },
};

export function PageHeader({ title, subtitle, actions }) {
  return (
    <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, mb: 3, flexWrap: "wrap" }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="h2" component="h1">
          {title}
        </Typography>
        {subtitle && (
          <Typography variant="body2" sx={{ mt: 0.5 }}>
            {subtitle}
          </Typography>
        )}
      </Box>
      {actions && <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>{actions}</Box>}
    </Box>
  );
}

export function Panel({ title, subtitle, actions, children, sx }) {
  return (
    <Paper elevation={0} sx={{ borderRadius: 2, ...sx }}>
      {(title || actions) && (
        <Box sx={{ px: 3, py: 2, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 2, borderBottom: "1px solid #e4e4e7" }}>
          <Box>
            <Typography sx={{ fontWeight: 600 }}>{title}</Typography>
            {subtitle && <Typography variant="body2">{subtitle}</Typography>}
          </Box>
          {actions && <Box sx={{ display: "flex", gap: 1 }}>{actions}</Box>}
        </Box>
      )}
      {children}
    </Paper>
  );
}

const STATUS_STYLE = {
  draft: { label: "Draft", variant: "outlined" },
  published: { label: "Published", variant: "filled" },
  archived: { label: "Archived", variant: "outlined" },
  concluded: { label: "Concluded", variant: "outlined" },
};

export function StatusChip({ status }) {
  const s = STATUS_STYLE[status] ?? { label: status, variant: "outlined" };
  return (
    <Chip
      size="small"
      label={s.label}
      variant={s.variant}
      sx={s.variant === "filled" ? { bgcolor: "#09090b", color: "#ffffff" } : { borderColor: "#d4d4d8", color: "#52525b" }}
    />
  );
}

/** Top-level message plus field-level messages from a 422 response. */
export function FormErrors({ error }) {
  if (!error) return null;
  const fields = Object.entries(error.fields ?? {});
  return (
    <Box sx={{ mb: 2, p: 1.5, border: "1px solid #09090b", borderRadius: 1.5 }} role="alert">
      <Typography sx={{ fontWeight: 600, fontSize: "0.8125rem" }}>{error.message}</Typography>
      {fields.length > 0 && (
        <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
          {fields.map(([path, msg]) => (
            <Typography component="li" key={path} variant="body2">
              {msg}
            </Typography>
          ))}
        </Box>
      )}
    </Box>
  );
}

export function Loading() {
  return (
    <Box sx={{ py: 8, display: "flex", justifyContent: "center" }}>
      <CircularProgress size={24} sx={{ color: "#09090b" }} />
    </Box>
  );
}

export function EmptyState({ title, children, action }) {
  return (
    <Box sx={{ py: 6, px: 3, textAlign: "center" }}>
      <Typography sx={{ fontWeight: 600 }}>{title}</Typography>
      {children && (
        <Typography variant="body2" sx={{ mt: 0.5 }}>
          {children}
        </Typography>
      )}
      {action && <Box sx={{ mt: 2 }}>{action}</Box>}
    </Box>
  );
}

export function ConfirmDialog({ open, title, children, confirmLabel = "Confirm", busy, onConfirm, onClose }) {
  return (
    <Dialog open={open} onClose={() => !busy && onClose()} maxWidth="xs" fullWidth slotProps={dialogPaper}>
      <DialogTitle sx={{ fontSize: "1.125rem", fontWeight: 600 }}>{title}</DialogTitle>
      <DialogContent>
        <Typography variant="body2">{children}</Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="contained" onClick={onConfirm} disabled={busy}>
          {busy ? <CircularProgress size={18} color="inherit" /> : confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** "2026-10-23T09:30:00Z" <-> value for <input type="datetime-local"> (local time). */
export function toLocalInput(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fromLocalInput(value) {
  return value ? new Date(value).toISOString() : null;
}

export function formatDateTime(iso) {
  return iso ? new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : "—";
}
