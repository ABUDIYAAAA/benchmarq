"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";
import ExamMessage from "@/components/exam/ExamMessage";
import ExamShell from "@/components/exam/ExamShell";
import SystemCheck from "@/components/exam/SystemCheck";
import WaitingRoom from "@/components/exam/WaitingRoom";
import { useExamRuntime } from "@/hooks/exam/useExamRuntime";
import { END_REASON_TEXT } from "@/lib/exam/bridge";

/**
 * Exam route, loaded by the Benchmarq Exam desktop app after its system
 * checks pass. It never calls the API directly: everything goes through the
 * desktop app's preload bridge, which holds the session token.
 */
export default function ExamPage() {
  const runtime = useExamRuntime();
  const { phase, boot, system, state, attempt, actions } = runtime;

  switch (phase) {
    case "unsupported":
      return (
        <ExamMessage title="Open this exam in Benchmarq Exam">
          Exams run only in the Benchmarq Exam desktop app. Go back to the portal and choose Launch exam.
        </ExamMessage>
      );

    case "loading":
      return (
        <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center" }}>
          <CircularProgress size={28} />
        </Box>
      );

    case "no_session": {
      const error = runtime.invalid?.message ?? boot?.error?.message;
      return (
        <ExamMessage
          title={error ? "This exam could not be opened" : "Open your exam from the browser"}
          actions={
            <>
              <Button variant="contained" onClick={actions.openPortal}>
                Open portal
              </Button>
              <Button variant="outlined" onClick={actions.quit}>
                Quit
              </Button>
            </>
          }
        >
          {error
            ? `${error} Launch the exam again from the candidate portal.`
            : "Benchmarq Exam opens exams from the candidate portal. Sign in in your browser, go to My exams and choose Launch exam."}
        </ExamMessage>
      );
    }

    case "system_check":
      return <SystemCheck apps={system.apps} actions={actions} />;

    case "waiting":
      return <WaitingRoom state={state} attempt={attempt} actions={actions} />;

    case "exam":
      return <ExamShell runtime={runtime} />;

    case "ended":
      return (
        <ExamMessage
          title="Exam finished"
          actions={
            <Button variant="contained" onClick={actions.quit}>
              Close Benchmarq Exam
            </Button>
          }
        >
          <Typography>{END_REASON_TEXT[attempt?.ended_reason] ?? "Your attempt has ended."}</Typography>
          <Typography variant="body2" sx={{ mt: 1 }}>
            You answered {attempt?.answered ?? 0} of {attempt?.total ?? 0} questions.
            {attempt?.submitted_at && ` Submitted at ${new Date(attempt.submitted_at).toLocaleString()}.`} Results will be shared according to the
            exam&apos;s rules.
          </Typography>
        </ExamMessage>
      );

    default:
      return null;
  }
}
