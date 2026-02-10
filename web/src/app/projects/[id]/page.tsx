"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  secrets as secretsApi,
  projects as projectsApi,
  type Secret,
  type Project,
  type PutSecretRequest,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { SecretTree } from "@/components/secret-tree";
import { Button } from "@/components/ui/button";
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
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Plus,
  Loader2,
  Key,
  ChevronLeft,
  Lock,
  Trash2,
  FolderTree,
  List,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";
import Link from "next/link";

export default function ProjectSecretsPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params.id as string;

  const [project, setProject] = useState<Project | null>(null);
  const [secretList, setSecretList] = useState<Secret[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<"tree" | "list">("tree");

  const [newPath, setNewPath] = useState("");
  const [newValue, setNewValue] = useState("");
  const [newDescription, setNewDescription] = useState("");

  const loadProject = async (): Promise<Project | null> => {
    try {
      const allProjects = await projectsApi.list();
      const p = (allProjects || []).find((p) => p.id === projectId);
      setProject(p || null);
      return p || null;
    } catch (err) {
      console.error(err);
      return null;
    }
  };

  const loadSecrets = async (projectName: string) => {
    try {
      const data = await secretsApi.list(projectName);
      setSecretList(data || []);
    } catch (err) {
      toast.error("Failed to load secrets");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    (async () => {
      const p = await loadProject();
      if (p?.name) {
        await loadSecrets(p.name);
      } else {
        setLoading(false);
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!project?.name) {
      toast.error("Project not loaded yet");
      return;
    }
    setCreating(true);
    try {
      const body: PutSecretRequest = { value: newValue };
      if (newDescription) {
        body.description = newDescription;
      }
      await secretsApi.put(project.name, newPath, body);
      toast.success(`Secret "${newPath}" created`);
      setDialogOpen(false);
      setNewPath("");
      setNewValue("");
      setNewDescription("");
      loadSecrets(project.name);
    } catch (err) {
      toast.error("Failed to create secret");
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (path: string) => {
    if (!project?.name) return;
    setDeleting(path);
    try {
      await secretsApi.delete(project.name, path);
      toast.success(`Secret "${path}" deleted`);
      loadSecrets(project.name);
    } catch (err) {
      toast.error("Failed to delete secret");
      console.error(err);
    } finally {
      setDeleting(null);
    }
  };

  const handleSelectSecret = (secret: Secret) => {
    router.push(
      `/projects/${projectId}/secrets/${encodeURIComponent(secret.path)}`
    );
  };

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <Link
            href="/dashboard"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ChevronLeft className="h-4 w-4" />
            Back to projects
          </Link>

          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                <Lock className="h-6 w-6 text-primary" />
                {project?.name || "Project"}
              </h1>
              {project?.description && (
                <p className="text-muted-foreground mt-1">
                  {project.description}
                </p>
              )}
            </div>

            <div className="flex items-center gap-2">
              {/* View mode toggle */}
              <div className="flex border rounded-md">
                <Button
                  variant={viewMode === "tree" ? "secondary" : "ghost"}
                  size="sm"
                  className="h-9 px-3 rounded-r-none"
                  onClick={() => setViewMode("tree")}
                >
                  <FolderTree className="h-4 w-4" />
                </Button>
                <Button
                  variant={viewMode === "list" ? "secondary" : "ghost"}
                  size="sm"
                  className="h-9 px-3 rounded-l-none"
                  onClick={() => setViewMode("list")}
                >
                  <List className="h-4 w-4" />
                </Button>
              </div>

              <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    New Secret
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <form onSubmit={handleCreate}>
                    <DialogHeader>
                      <DialogTitle>Create Secret</DialogTitle>
                      <DialogDescription>
                        Add a new secret to this project
                      </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                      <div className="space-y-2">
                        <Label htmlFor="secret-path">Path</Label>
                        <Input
                          id="secret-path"
                          placeholder="services/payment/prod/STRIPE_KEY"
                          value={newPath}
                          onChange={(e) => setNewPath(e.target.value)}
                          required
                          autoFocus
                          className="font-mono"
                        />
                        <p className="text-xs text-muted-foreground">
                          Use forward slashes to create folders: services/payment/prod/STRIPE_KEY
                        </p>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="secret-value">Value</Label>
                        <Textarea
                          id="secret-value"
                          placeholder="sk_live_xxxxxxxxxxxxx"
                          value={newValue}
                          onChange={(e) => setNewValue(e.target.value)}
                          required
                          rows={3}
                          className="font-mono"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="secret-desc">
                          Description (optional)
                        </Label>
                        <Input
                          id="secret-desc"
                          placeholder="Stripe production API key"
                          value={newDescription}
                          onChange={(e) => setNewDescription(e.target.value)}
                        />
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
                      <Button type="submit" disabled={creating}>
                        {creating ? (
                          <>
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            Creating…
                          </>
                        ) : (
                          "Create Secret"
                        )}
                      </Button>
                    </DialogFooter>
                  </form>
                </DialogContent>
              </Dialog>
            </div>
          </div>
        </div>

        {/* Secrets content */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : secretList.length === 0 ? (
          <div className="border border-dashed rounded-lg flex flex-col items-center justify-center py-16 text-center">
            <Key className="h-12 w-12 text-muted-foreground/50 mb-4" />
            <h3 className="text-lg font-medium mb-1">No secrets yet</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Create your first secret in this project
            </p>
            <Button onClick={() => setDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              New Secret
            </Button>
          </div>
        ) : viewMode === "tree" ? (
          /* ─── FOLDER TREE VIEW ─── */
          <SecretTree
            secrets={secretList}
            onSelectSecret={handleSelectSecret}
          />
        ) : (
          /* ─── LIST VIEW (original) ─── */
          <div className="border rounded-lg">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[40%]">Path</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Version</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-[50px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {secretList.map((secret) => (
                  <TableRow
                    key={secret.id}
                    className="cursor-pointer"
                    onClick={() =>
                      router.push(
                        `/projects/${projectId}/secrets/${encodeURIComponent(
                          secret.path
                        )}`
                      )
                    }
                  >
                    <TableCell className="font-mono text-sm">
                      <div className="flex items-center gap-2">
                        <Key className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                        {secret.path}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {secret.description || "—"}
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="font-mono text-xs">
                        v{secret.current_version || 1}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDistanceToNow(new Date(secret.created_at), {
                        addSuffix: true,
                      })}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDelete(secret.path);
                        }}
                        disabled={deleting === secret.path}
                      >
                        {deleting === secret.path ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Trash2 className="h-4 w-4" />
                        )}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </AppShell>
  );
}
