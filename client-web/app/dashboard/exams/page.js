"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Container from "@mui/material/Container";
import MenuItem from "@mui/material/MenuItem";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import CreateExamDialog from "@/components/exams/CreateExamDialog";
import { EmptyState, Loading, PageHeader, Panel, StatusChip, formatDateTime } from "@/components/exams/common";
import { useOrg } from "@/context/OrgContext";
import { useExamList } from "@/hooks/api/useExams";
import { can } from "@/lib/permissions";

export default function ExamsPage() {
  const { currentOrg } = useOrg();
  const router = useRouter();
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [cursors, setCursors] = useState([]); // stack of cursors for previous pages
  const [creating, setCreating] = useState(false);

  const role = currentOrg?.my_role;
  const cursor = cursors[cursors.length - 1];
  const { data, isLoading } = useExamList(can(role, "viewExams") ? currentOrg?.id : null, {
    status: status || undefined,
    q: search.trim() || undefined,
    cursor,
    limit: 20,
  });

  if (currentOrg && !can(role, "viewExams")) {
    return (
      <Container maxWidth="lg" sx={{ py: 4 }}>
        <EmptyState title="No access to exams">Your role in {currentOrg.name} does not include managing exams.</EmptyState>
      </Container>
    );
  }

  const items = data?.items ?? [];
  const resetPaging = () => setCursors([]);

  return (
    <Container maxWidth="lg" sx={{ py: 4, px: { xs: 2.5, sm: 4 } }}>
      <PageHeader
        title="Exams"
        subtitle={currentOrg ? `Exams in ${currentOrg.name}` : undefined}
        actions={
          can(role, "authorExams") && (
            <Button variant="contained" onClick={() => setCreating(true)}>
              New exam
            </Button>
          )
        }
      />

      <Panel>
        <Box sx={{ display: "flex", gap: 2, p: 2, borderBottom: "1px solid #e4e4e7" }}>
          <TextField
            size="small"
            placeholder="Search by title or code"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              resetPaging();
            }}
            sx={{ flex: 1, maxWidth: 360 }}
          />
          <TextField
            size="small"
            select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              resetPaging();
            }}
            sx={{ minWidth: 160 }}
            slotProps={{ select: { displayEmpty: true } }}
          >
            <MenuItem value="">All statuses</MenuItem>
            <MenuItem value="draft">Draft</MenuItem>
            <MenuItem value="published">Published</MenuItem>
            <MenuItem value="archived">Archived</MenuItem>
          </TextField>
        </Box>

        {isLoading ? (
          <Loading />
        ) : items.length === 0 ? (
          <EmptyState
            title={search || status ? "No matching exams" : "No exams yet"}
            action={
              !search && !status && can(role, "authorExams") ? (
                <Button variant="outlined" onClick={() => setCreating(true)}>
                  Create your first exam
                </Button>
              ) : null
            }
          >
            {!search && !status && "Create an exam, add sections and questions, then publish it and assign candidates."}
          </EmptyState>
        ) : (
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Title</TableCell>
                <TableCell>Code</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Total marks</TableCell>
                <TableCell>Created</TableCell>
                <TableCell align="right" />
              </TableRow>
            </TableHead>
            <TableBody>
              {items.map((exam) => (
                <TableRow key={exam.id} hover onClick={() => router.push(`/dashboard/exams/${exam.id}`)} sx={{ cursor: "pointer" }}>
                  <TableCell>
                    <Link href={`/dashboard/exams/${exam.id}`} style={{ color: "inherit", fontWeight: 500 }}>
                      {exam.title}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">{exam.code ?? "—"}</Typography>
                  </TableCell>
                  <TableCell>
                    <StatusChip status={exam.status} />
                  </TableCell>
                  <TableCell align="right">{exam.status === "draft" ? "—" : exam.total_marks}</TableCell>
                  <TableCell>
                    <Typography variant="body2">{formatDateTime(exam.created_at)}</Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Button size="small" variant="outlined" component={Link} href={`/dashboard/exams/${exam.id}`} onClick={(e) => e.stopPropagation()}>
                      {exam.status === "draft" && can(role, "authorExams") ? "Edit" : "Open"}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {(cursors.length > 0 || data?.next_cursor) && (
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1, p: 2, borderTop: "1px solid #e4e4e7" }}>
            <Button variant="outlined" size="small" disabled={cursors.length === 0} onClick={() => setCursors((c) => c.slice(0, -1))}>
              Previous
            </Button>
            <Button variant="outlined" size="small" disabled={!data?.next_cursor} onClick={() => setCursors((c) => [...c, data.next_cursor])}>
              Next
            </Button>
          </Box>
        )}
      </Panel>

      {currentOrg && <CreateExamDialog open={creating} onClose={() => setCreating(false)} orgId={currentOrg.id} />}
    </Container>
  );
}
