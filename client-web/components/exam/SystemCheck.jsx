"use client";

import { useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";
import ExamMessage from "@/components/exam/ExamMessage";

/**
 * Shown before the exam when blocked applications are running. Closing them
 * restarts Benchmarq Exam, which then reopens this exam on its own.
 */
export default function SystemCheck({ apps, actions }) {
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState(null);

  const run = (name, fn) => async () => {
    setBusy(name);
    setError(null);
    try {
      await fn();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <ExamMessage
      title="Close blocked applications"
      actions={
        <>
          <Button variant="contained" disabled={Boolean(busy)} onClick={run("close", actions.closeApps)}>
            {busy === "close" ? "Closing..." : "Close apps and restart"}
          </Button>
          <Button variant="outlined" disabled={Boolean(busy)} onClick={run("check", actions.checkSystem)}>
            {busy === "check" ? "Checking..." : "Check again"}
          </Button>
          <Button color="inherit" disabled={Boolean(busy)} onClick={actions.quit}>
            Quit
          </Button>
        </>
      }
    >
      <Typography variant="body2">
        These applications are not allowed during the exam and must be closed before it can open.
      </Typography>
      <List dense disablePadding sx={{ my: 1.5, border: 1, borderColor: "divider", borderRadius: 1 }}>
        {apps.map((name) => (
          <ListItem key={name} divider>
            <ListItemText primary={name} primaryTypographyProps={{ color: "text.primary", fontSize: "0.875rem" }} />
          </ListItem>
        ))}
      </List>
      <Typography variant="body2">
        Save your work first: closing an app this way discards unsaved changes. Benchmarq Exam restarts afterwards and reopens your exam. You can
        also close them yourself and choose Check again.
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mt: 2 }}>
          {error}
        </Alert>
      )}
    </ExamMessage>
  );
}
