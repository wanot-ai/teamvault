"use client";

import { useEffect, useState, useCallback } from "react";
import {
  leases as leasesApi,
  projects as projectsApi,
  type Lease,
  type LeaseStatus,
  type Project,
  type IssueLeaseRequest,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import {
  KeyRound,
  Plus,
  Loader2,
  XCircle,
  CheckCircle2,
  Clock,
  AlertTriangle,
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";
import { format, formatDistanceToNow } from "date-fns";

function StatusBadge({ status }: { status: LeaseStatus }) {
  switch (status) {
    case "active":
      return (
        <Badge
          variant="secondary"
          className="gap-1 text-green-400 border-green-500/20 bg-green-500/10"
        >
          <CheckCircle2 className="h-3 w-3" />
          Active
        </Badge>
      );
    case "expired":
      return (
        <Badge
          variant="secondary"
          className="gap-1 text-yellow-400 border-yellow-500/20 bg-yellow-500/10"
        >
          <Clock className="h-3 w-3" />
          Expired
        </Badge>
      );
    case "revoked":
      return (
        <Badge
          variant="secondary"
          className="gap-1 text-red-400 border-red-500/20 bg-red-500/10"
        >
          <XCircle className="h-3 w-3" />
          Revoked
        </Badge>
      );
    default:
      return <Badge variant="secondary">{status}</Badge>;
  }
}

export default function LeasesPage() {
  const [leaseList, setLeaseList] = useState<Lease[]>([]);
  const [projectList, setProjectList] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const [form, setForm] = useState<IssueLeaseRequest>({
    type: "database",
    ttl_seconds: 3600,
    project_id: "",
  });

  const loadLeases = useCallback(async () => {
    try {
      const data = await leasesApi.list();
      setLeaseList(data || []);
    } catch (err) {
      toast.error("Failed to load leases");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadProjects = useCallback(async () => {
    try {
      const data = await projectsApi.list();
      setProjectList(data || []);
    } catch (err) {
      console.error(err);
    }
  }, []);

  useEffect(() => {
    loadLeases();
    loadProjects();
  }, [loadLeases, loadProjects]);

  // Auto-refresh every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      loadLeases();
    }, 30000);
    return () => clearInterval(interval);
  }, [loadLeases]);

  const handleIssue = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      await leasesApi.issue(form);
      toast.success("Lease issued successfully");
      setDialogOpen(false);
      setForm({ type: "database", ttl_seconds: 3600, project_id: "" });
      loadLeases();
    } catch (err) {
      toast.error("Failed to issue lease");
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const handleRevoke = async (leaseId: string) => {
    setRevokingId(leaseId);
    try {
      await leasesApi.revoke(leaseId);
      toast.success("Lease revoked");
      loadLeases();
    } catch (err) {
      toast.error("Failed to revoke lease");
      console.error(err);
    } finally {
      setRevokingId(null);
    }
  };

  const activeLeasesCount = leaseList.filter((l) => l.status === "active").length;

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
              <KeyRound className="h-6 w-6 text-primary" />
              Leases
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage dynamic secret leases · Auto-refreshes every 30s
              {activeLeasesCount > 0 && (
                <span className="ml-2">
                  ·{" "}
                  <span className="text-green-400 font-medium">
                    {activeLeasesCount} active
                  </span>
                </span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => loadLeases()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Refresh
            </Button>
            <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
              <DialogTrigger asChild>
                <Button>
                  <Plus className="mr-2 h-4 w-4" />
                  Issue Lease
                </Button>
              </DialogTrigger>
              <DialogContent>
                <form onSubmit={handleIssue}>
                  <DialogHeader>
                    <DialogTitle>Issue Lease</DialogTitle>
                    <DialogDescription>
                      Create a new dynamic secret lease with a TTL
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>Type</Label>
                      <Select
                        value={form.type}
                        onValueChange={(v) =>
                          setForm((f) => ({
                            ...f,
                            type: v as "database",
                          }))
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="database">Database</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Project</Label>
                      <Select
                        value={form.project_id}
                        onValueChange={(v) =>
                          setForm((f) => ({ ...f, project_id: v }))
                        }
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Select a project" />
                        </SelectTrigger>
                        <SelectContent>
                          {projectList.map((p) => (
                            <SelectItem key={p.id} value={p.id}>
                              {p.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ttl">TTL (seconds)</Label>
                      <Input
                        id="ttl"
                        type="number"
                        min={60}
                        max={86400}
                        value={form.ttl_seconds}
                        onChange={(e) =>
                          setForm((f) => ({
                            ...f,
                            ttl_seconds: parseInt(e.target.value) || 3600,
                          }))
                        }
                        required
                      />
                      <p className="text-xs text-muted-foreground">
                        Duration in seconds (60s – 86400s). Default: 1 hour (3600s)
                      </p>
                    </div>
                  </div>
                  <DialogFooter>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setDialogOpen(false)}
                    >
                      Cancel
                    </Button>
                    <Button type="submit" disabled={creating || !form.project_id}>
                      {creating ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          Issuing…
                        </>
                      ) : (
                        "Issue Lease"
                      )}
                    </Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>
        </div>

        {/* Leases Table */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : leaseList.length === 0 ? (
          <Card className="border-dashed">
            <CardContent className="flex flex-col items-center justify-center py-16 text-center">
              <KeyRound className="h-12 w-12 text-muted-foreground/50 mb-4" />
              <h3 className="text-lg font-medium mb-1">No leases</h3>
              <p className="text-sm text-muted-foreground mb-4">
                Issue a lease to provide time-limited access to dynamic secrets
              </p>
              <Button onClick={() => setDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Issue Lease
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="border rounded-lg">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Issued To</TableHead>
                  <TableHead>Path</TableHead>
                  <TableHead>Issued At</TableHead>
                  <TableHead>Expires At</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-[80px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {leaseList.map((lease) => (
                  <TableRow key={lease.id}>
                    <TableCell className="font-mono text-xs">
                      {lease.id.slice(0, 8)}…
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs capitalize">
                        {lease.type}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {lease.issued_to}
                    </TableCell>
                    <TableCell className="font-mono text-xs max-w-[200px] truncate">
                      {lease.path || "—"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {format(new Date(lease.issued_at), "MMM d, HH:mm")}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {format(new Date(lease.expires_at), "MMM d, HH:mm")}
                      <span className="block text-xs">
                        ({formatDistanceToNow(new Date(lease.expires_at), { addSuffix: true })})
                      </span>
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={lease.status} />
                    </TableCell>
                    <TableCell>
                      {lease.status === "active" && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-8 text-xs text-destructive hover:text-destructive"
                          onClick={() => handleRevoke(lease.id)}
                          disabled={revokingId === lease.id}
                        >
                          {revokingId === lease.id ? (
                            <Loader2 className="h-3 w-3 animate-spin" />
                          ) : (
                            "Revoke"
                          )}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Count */}
        {!loading && leaseList.length > 0 && (
          <p className="text-xs text-muted-foreground text-right">
            Showing {leaseList.length} lease{leaseList.length !== 1 && "s"}
          </p>
        )}
      </div>
    </AppShell>
  );
}
