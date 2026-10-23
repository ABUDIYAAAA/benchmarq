"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Container from "@mui/material/Container";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import { EmptyState, Loading, PageHeader, Panel, formatDateTime } from "@/components/exams/common";
import { useToast } from "@/context/ToastContext";
import { apiError, useMyExams } from "@/hooks/api/useExams";
import examsApi from "@/lib/api/exams.api";

function availabilityText(a) {
  switch (a.state) {
    case "upcoming":
      return a.opens_at ? `Opens ${formatDateTime(a.opens_at)}` : "Not open yet";
    case "open":
      return a.closes_at ? `Open until ${formatDateTime(a.closes_at)}` : "Open";
    case "in_progress":
      return "In progress";
    case "closed":
      return a.reason ? `Closed (${a.reason.replaceAll("_", " ")})` : "Closed";
    default:
      return a.state;
  }
}

/**
 * Exams assigned to the signed-in user, across every organization. Exams
 * are taken in the Benchmarq Exam desktop app: Launch issues a one-time code
 * and opens the app through its benchmarq:// link.
 */
export default function MyExamsPage() {
  const { showError } = useToast();
  const { data: exams = [], isLoading } = useMyExams();
  const [launching, setLaunching] = useState(null);
  const [launched, setLaunched] = useState(null);

  const launch = async (exam) => {
    setLaunching(exam.exam_id);
    try {
      const grant = await examsApi.launch(exam.exam_id);
      setLaunched(exam.title);
      window.location.assign(grant.launch_url); // opens the desktop app via benchmarq://
    } catch (err) {
      showError(apiError(err, "Could not launch the exam.").message);
    } finally {
      setLaunching(null);
    }
  };

  return (
    <Container maxWidth="lg" sx={{ py: 4, px: { xs: 2.5, sm: 4 } }}>
      <PageHeader title="My exams" subtitle="Exams you have been assigned. They open in the Benchmarq Exam desktop app." />

      {launched && (
        <Panel sx={{ mb: 3 }}>
          <Box sx={{ px: 3, py: 2 }}>
            <Typography sx={{ fontWeight: 600 }}>Opening {launched} in Benchmarq Exam</Typography>
            <Typography variant="body2" sx={{ mt: 0.5 }}>
              If nothing happens, install Benchmarq Exam and press Launch again. Each launch link works once and expires after a few minutes.
            </Typography>
          </Box>
        </Panel>
      )}

      <Panel>
        {isLoading ? (
          <Loading />
        ) : exams.length === 0 ? (
          <EmptyState title="No exams assigned">When an organization assigns you an exam, it appears here.</EmptyState>
        ) : (
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Exam</TableCell>
                <TableCell>Organization</TableCell>
                <TableCell>Availability</TableCell>
                <TableCell>Duration</TableCell>
                <TableCell>Attempts</TableCell>
                <TableCell align="right" />
              </TableRow>
            </TableHead>
            <TableBody>
              {exams.map((e) => {
                const a = e.availability;
                return (
                  <TableRow key={e.exam_id} hover>
                    <TableCell>
                      <Typography sx={{ fontSize: "0.875rem", fontWeight: 500 }}>{e.title}</Typography>
                      {e.description && (
                        <Typography variant="body2" noWrap sx={{ maxWidth: 320 }}>
                          {e.description}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{e.org_name}</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{availabilityText(a)}</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{a.duration_minutes} min</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {a.attempts_used} of {a.max_attempts}
                      </Typography>
                    </TableCell>
                    <TableCell align="right">
                      <Button variant={a.can_launch ? "contained" : "outlined"} size="small" disabled={!a.can_launch || launching === e.exam_id} onClick={() => launch(e)}>
                        {launching === e.exam_id ? <CircularProgress size={16} color="inherit" /> : a.state === "in_progress" ? "Resume" : "Launch"}
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
      </Panel>
    </Container>
  );
}
