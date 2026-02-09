"use client";

import { useEffect, useState, useCallback } from "react";
import { audit as auditApi, type AuditEvent, type AuditFilters } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
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
  ScrollText,
  Loader2,
  Filter,
  RotateCcw,
  CheckCircle2,
  XCircle,
  AlertTriangle,
} from "lucide-react";
import { format } from "date-fns";
import { toast } from "sonner";

const ACTION_OPTIONS = [
  { value: "", label: "All Actions" },
  { value: "secret.read", label: "Secret Read" },
  { value: "secret.write", label: "Secret Write" },
  { value: "secret.delete", label: "Secret Delete" },
  { value: "project.create", label: "Project Create" },
  { value: "auth.login", label: "Login" },
  { value: "sa.create", label: "SA Create" },
  { value: "policy.create", label: "Policy Create" },
];

const OUTCOME_OPTIONS = [
  { value: "", label: "All Outcomes" },
  { value: "success", label: "Success" },
  { value: "denied", label: "Denied" },
  { value: "error", label: "Error" },
];

function OutcomeBadge({ outcome }: { outcome: string }) {
  switch (outcome) {
    case "success":
      return (
        <Badge variant="secondary" className="gap-1 text-green-400 border-green-500/20 bg-green-500/10">
          <CheckCircle2 className="h-3 w-3" />
          Success
        </Badge>
      );
    case "denied":
      return (
        <Badge variant="secondary" className="gap-1 text-yellow-400 border-yellow-500/20 bg-yellow-500/10">
          <AlertTriangle className="h-3 w-3" />
          Denied
        </Badge>
      );
    case "error":
      return (
        <Badge variant="secondary" className="gap-1 text-red-400 border-red-500/20 bg-red-500/10">
          <XCircle className="h-3 w-3" />
          Error
        </Badge>
      );
    default:
      return <Badge variant="secondary">{outcome}</Badge>;
  }
}

export default function AuditPage() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState<AuditFilters>({
    action: "",
    outcome: "",
    actor_id: "",
    from: "",
    to: "",
    limit: 100,
  });

  const loadEvents = useCallback(async () => {
    setLoading(true);
    try {
      const data = await auditApi.list(filters);
      setEvents(data || []);
    } catch (err) {
      toast.error("Failed to load audit events");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [filters]);

  useEffect(() => {
    loadEvents();
  }, [loadEvents]);

  const resetFilters = () => {
    setFilters({
      action: "",
      outcome: "",
      actor_id: "",
      from: "",
      to: "",
      limit: 100,
    });
  };

  const hasActiveFilters =
    filters.action || filters.outcome || filters.actor_id || filters.from || filters.to;

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
            <ScrollText className="h-6 w-6 text-primary" />
            Audit Log
          </h1>
          <p className="text-muted-foreground mt-1">
            Track all actions across your vault
          </p>
        </div>

        {/* Filters */}
        <div className="border rounded-lg p-4 space-y-4 bg-card/50">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Filter className="h-4 w-4" />
            Filters
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
            <div className="space-y-1.5">
              <Label className="text-xs">Action</Label>
              <Select
                value={filters.action || ""}
                onValueChange={(v) =>
                  setFilters((f) => ({ ...f, action: v }))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="All Actions" />
                </SelectTrigger>
                <SelectContent>
                  {ACTION_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value || "__all__"}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Outcome</Label>
              <Select
                value={filters.outcome || ""}
                onValueChange={(v) =>
                  setFilters((f) => ({ ...f, outcome: v }))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="All Outcomes" />
                </SelectTrigger>
                <SelectContent>
                  {OUTCOME_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value || "__all__"}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Actor ID</Label>
              <Input
                placeholder="Filter by actor…"
                value={filters.actor_id || ""}
                onChange={(e) =>
                  setFilters((f) => ({ ...f, actor_id: e.target.value }))
                }
                className="font-mono text-xs"
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">From Date</Label>
              <Input
                type="datetime-local"
                value={filters.from || ""}
                onChange={(e) =>
                  setFilters((f) => ({ ...f, from: e.target.value }))
                }
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">To Date</Label>
              <Input
                type="datetime-local"
                value={filters.to || ""}
                onChange={(e) =>
                  setFilters((f) => ({ ...f, to: e.target.value }))
                }
              />
            </div>
          </div>

          {hasActiveFilters && (
            <Button variant="ghost" size="sm" onClick={resetFilters} className="gap-1">
              <RotateCcw className="h-3 w-3" />
              Reset filters
            </Button>
          )}
        </div>

        {/* Events Table */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : events.length === 0 ? (
          <div className="border border-dashed rounded-lg flex flex-col items-center justify-center py-16 text-center">
            <ScrollText className="h-12 w-12 text-muted-foreground/50 mb-4" />
            <h3 className="text-lg font-medium mb-1">No audit events</h3>
            <p className="text-sm text-muted-foreground">
              {hasActiveFilters
                ? "Try adjusting your filters"
                : "Events will appear as actions occur"}
            </p>
          </div>
        ) : (
          <div className="border rounded-lg">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Actor</TableHead>
                  <TableHead>Resource</TableHead>
                  <TableHead>Outcome</TableHead>
                  <TableHead>IP</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((event) => (
                  <TableRow key={event.id}>
                    <TableCell className="text-sm whitespace-nowrap">
                      {format(new Date(event.timestamp), "MMM d, HH:mm:ss")}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-xs">
                        {event.action}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col">
                        <span className="text-xs text-muted-foreground capitalize">
                          {event.actor_type}
                        </span>
                        <span className="font-mono text-xs truncate max-w-[120px]">
                          {event.actor_id}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs truncate max-w-[200px]">
                      {event.resource}
                    </TableCell>
                    <TableCell>
                      <OutcomeBadge outcome={event.outcome} />
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {event.ip || "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Results count */}
        {!loading && events.length > 0 && (
          <p className="text-xs text-muted-foreground text-right">
            Showing {events.length} event{events.length !== 1 && "s"}
          </p>
        )}
      </div>
    </AppShell>
  );
}
