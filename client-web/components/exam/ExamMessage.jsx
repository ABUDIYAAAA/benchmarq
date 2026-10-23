"use client";

import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";

/** Centered card for loading, error and finished states. */
export default function ExamMessage({ title, children, actions }) {
  return (
    <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", p: 3, bgcolor: "#fafafa" }}>
      <Paper elevation={0} sx={{ width: "100%", maxWidth: 480, p: 4, borderRadius: 2 }}>
        <Typography variant="h3" component="h1">
          {title}
        </Typography>
        <Box sx={{ mt: 1.5, color: "text.secondary", fontSize: "0.875rem", lineHeight: 1.6 }}>{children}</Box>
        {actions && (
          <Stack direction="row" spacing={1.5} sx={{ mt: 3 }}>
            {actions}
          </Stack>
        )}
      </Paper>
    </Box>
  );
}
