"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  secrets as secretsApi,
  projects as projectsApi,
  type SecretValue,
  type SecretVersion,
  type Project,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  ChevronLeft,
  Eye,
  EyeOff,
  Copy,
  Check,
  Key,
  Clock,
  Loader2,
  Pencil,
} from "lucide-react";
import { toast } from "sonner";
import { format, formatDistanceToNow } from "date-fns";
import { useCopyToClipboard } from "@/lib/hooks";
import Link from "next/link";

export default function SecretDetailPage() {
  const params = useParams();
  const projectId = params.id as string;
  const secretPath = decodeURIComponent(params.path as string);

  const [project, setProject] = useState<Project | null>(null);
  const [secret, setSecret] = useState<SecretValue | null>(null);
  const [versions, setVersions] = useState<SecretVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [revealed, setRevealed] = useState(false);
  const [updateOpen, setUpdateOpen] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [newValue, setNewValue] = useState("");
  const { copied, copy } = useCopyToClipboard();

  const loadData = useCallback(async () => {
    try {
      const [secretData, versionsData, allProjects] = await Promise.all([
        secretsApi.get(projectId, secretPath),
        secretsApi.versions(projectId, secretPath),
        projectsApi.list(),
      ]);
      setSecret(secretData);
      setVersions(versionsData || []);
      const p = (allProjects || []).find((p) => p.id === projectId);
      setProject(p || null);
    } catch (err) {
      toast.error("Failed to load secret");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [projectId, secretPath]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    setUpdating(true);
    try {
      await secretsApi.put(projectId, secretPath, { value: newValue });
      toast.success("Secret updated — new version created");
      setUpdateOpen(false);
      setNewValue("");
      setRevealed(false);
      loadData();
    } catch (err) {
      toast.error("Failed to update secret");
      console.error(err);
    } finally {
      setUpdating(false);
    }
  };

  const handleCopy = async () => {
    if (secret?.value) {
      await copy(secret.value);
      toast.success("Secret copied to clipboard");
    }
  };

  const maskedValue = "••••••••••••••••••••••••••••••••";

  if (loading) {
    return (
      <AppShell>
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </AppShell>
    );
  }

  if (!secret) {
    return (
      <AppShell>
        <div className="text-center py-20">
          <Key className="h-12 w-12 text-muted-foreground/50 mx-auto mb-4" />
          <h3 className="text-lg font-medium mb-1">Secret not found</h3>
          <Link href={`/projects/${projectId}`}>
            <Button variant="outline" className="mt-4">
              Back to project
            </Button>
          </Link>
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Breadcrumb */}
        <div>
          <Link
            href={`/projects/${projectId}`}
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ChevronLeft className="h-4 w-4" />
            {project?.name || "Project"} / Secrets
          </Link>

          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                <Key className="h-6 w-6 text-primary" />
                <span className="font-mono">{secretPath}</span>
              </h1>
              <p className="text-muted-foreground mt-1 text-sm">
                Version {secret.version} · Last updated{" "}
                {formatDistanceToNow(new Date(secret.created_at), {
                  addSuffix: true,
                })}
              </p>
            </div>

            <Dialog open={updateOpen} onOpenChange={setUpdateOpen}>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <Pencil className="mr-2 h-4 w-4" />
                  Update Value
                </Button>
              </DialogTrigger>
              <DialogContent>
                <form onSubmit={handleUpdate}>
                  <DialogHeader>
                    <DialogTitle>Update Secret</DialogTitle>
                    <DialogDescription>
                      This will create a new version. Previous versions are preserved.
                    </DialogDescription>
                  </DialogHeader>
                  <div className="py-4">
                    <Label htmlFor="update-value">New Value</Label>
                    <Textarea
                      id="update-value"
                      placeholder="Enter new secret value…"
                      value={newValue}
                      onChange={(e) => setNewValue(e.target.value)}
                      required
                      rows={4}
                      className="font-mono mt-2"
                    />
                  </div>
                  <DialogFooter>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setUpdateOpen(false)}
                    >
                      Cancel
                    </Button>
                    <Button type="submit" disabled={updating}>
                      {updating ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          Updating…
                        </>
                      ) : (
                        "Update Secret"
                      )}
                    </Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>
        </div>

        {/* Secret Value Card */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Secret Value</CardTitle>
            <CardDescription>
              Click the eye icon to reveal. Values are masked by default.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className="flex-1 bg-muted/50 border rounded-md px-4 py-3 font-mono text-sm overflow-x-auto">
                {revealed ? (
                  <span className="break-all select-all">{secret.value}</span>
                ) : (
                  <span className="text-muted-foreground">{maskedValue}</span>
                )}
              </div>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-9 w-9"
                  onClick={() => setRevealed(!revealed)}
                  title={revealed ? "Hide value" : "Reveal value"}
                >
                  {revealed ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-9 w-9"
                  onClick={handleCopy}
                  title="Copy to clipboard"
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-green-500" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Metadata */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Metadata</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
              <div>
                <dt className="text-muted-foreground mb-1">Path</dt>
                <dd className="font-mono">{secret.path}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground mb-1">Current Version</dt>
                <dd>
                  <Badge variant="secondary" className="font-mono">
                    v{secret.version}
                  </Badge>
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground mb-1">Last Updated</dt>
                <dd>{format(new Date(secret.created_at), "PPpp")}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground mb-1">Updated By</dt>
                <dd className="font-mono text-xs">{secret.created_by}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        {/* Version History */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Clock className="h-4 w-4" />
              Version History
            </CardTitle>
            <CardDescription>
              All versions of this secret are preserved
            </CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {versions.length === 0 ? (
              <div className="px-6 pb-6 text-sm text-muted-foreground">
                No version history available
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Version</TableHead>
                    <TableHead>Created By</TableHead>
                    <TableHead>Created At</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {versions
                    .sort((a, b) => b.version - a.version)
                    .map((v) => (
                      <TableRow key={v.id}>
                        <TableCell>
                          <Badge
                            variant={
                              v.version === secret.version
                                ? "default"
                                : "secondary"
                            }
                            className="font-mono"
                          >
                            v{v.version}
                            {v.version === secret.version && " (current)"}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {v.created_by}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {format(new Date(v.created_at), "PPpp")}
                        </TableCell>
                      </TableRow>
                    ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </AppShell>
  );
}
