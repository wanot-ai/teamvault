"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  secrets as secretsApi,
  projects as projectsApi,
  rotation as rotationApi,
  type SecretValue,
  type SecretVersion,
  type Project,
  type RotationConfig,
  type SetRotationRequest,
  type ConnectorType,
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
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
  RefreshCw,
  Calendar,
  Settings,
  GitCompare,
  Play,
  Power,
  PowerOff,
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

  // Rotation state
  const [rotationConfig, setRotationConfig] = useState<RotationConfig | null>(null);
  const [rotationLoading, setRotationLoading] = useState(true);
  const [rotationDialogOpen, setRotationDialogOpen] = useState(false);
  const [settingRotation, setSettingRotation] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [rotationForm, setRotationForm] = useState<SetRotationRequest>({
    schedule: "0 0 * * *",
    connector_type: "random_password",
    enabled: true,
  });

  // Diff state
  const [diffOpen, setDiffOpen] = useState(false);
  const [diffVersionA, setDiffVersionA] = useState<number | "">("");
  const [diffVersionB, setDiffVersionB] = useState<number | "">("");
  const [diffValueA, setDiffValueA] = useState<string | null>(null);
  const [diffValueB, setDiffValueB] = useState<string | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffRevealed, setDiffRevealed] = useState(false);

  // Resolve project name from UUID for API calls
  const [projectName, setProjectName] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      const allProjects = await projectsApi.list();
      const p = (allProjects || []).find((proj) => proj.id === projectId);
      setProject(p || null);
      const pName = p?.name;
      if (!pName) {
        toast.error("Project not found");
        setLoading(false);
        return;
      }
      setProjectName(pName);

      const [secretData, versionsData] = await Promise.all([
        secretsApi.get(pName, secretPath),
        secretsApi.versions(pName, secretPath),
      ]);
      setSecret(secretData);
      setVersions(versionsData || []);
    } catch (err) {
      toast.error("Failed to load secret");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [projectId, secretPath]);

  const loadRotation = useCallback(async () => {
    if (!projectName) return;
    setRotationLoading(true);
    try {
      const config = await rotationApi.get(projectName, secretPath);
      setRotationConfig(config);
      setRotationForm({
        schedule: config.schedule,
        connector_type: config.connector_type,
        enabled: config.enabled,
      });
    } catch {
      // No rotation configured — that's fine
      setRotationConfig(null);
    } finally {
      setRotationLoading(false);
    }
  }, [projectName, secretPath]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    if (projectName) loadRotation();
  }, [projectName, loadRotation]);

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!projectName) { toast.error("Project not loaded"); return; }
    setUpdating(true);
    try {
      await secretsApi.put(projectName, secretPath, { value: newValue });
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

  const handleSetRotation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!projectName) { toast.error("Project not loaded"); return; }
    setSettingRotation(true);
    try {
      await rotationApi.set(projectName, secretPath, rotationForm);
      toast.success("Rotation configuration saved");
      setRotationDialogOpen(false);
      loadRotation();
    } catch (err) {
      toast.error("Failed to set rotation");
      console.error(err);
    } finally {
      setSettingRotation(false);
    }
  };

  const handleRotateNow = async () => {
    if (!projectName) { toast.error("Project not loaded"); return; }
    setRotating(true);
    try {
      const result = await rotationApi.rotateNow(projectName, secretPath);
      toast.success(`Secret rotated — now at version ${result.version}`);
      loadData();
      loadRotation();
    } catch (err) {
      toast.error("Failed to rotate secret");
      console.error(err);
    } finally {
      setRotating(false);
    }
  };

  const handleCompareVersions = async () => {
    if (diffVersionA === "" || diffVersionB === "") {
      toast.error("Select two versions to compare");
      return;
    }
    if (!projectName) { toast.error("Project not loaded"); return; }
    setDiffLoading(true);
    setDiffRevealed(false);
    try {
      // Fetch both versions by getting the secret with version param
      // We'll fetch the secret value for each version
      const [valA, valB] = await Promise.all([
        secretsApi.get(projectName, `${secretPath}?version=${diffVersionA}`),
        secretsApi.get(projectName, `${secretPath}?version=${diffVersionB}`),
      ]);
      setDiffValueA(valA.value);
      setDiffValueB(valB.value);
    } catch {
      toast.error("Failed to load version data for comparison");
      setDiffValueA(null);
      setDiffValueB(null);
    } finally {
      setDiffLoading(false);
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

  const sortedVersions = [...versions].sort((a, b) => b.version - a.version);

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

        {/* Main Tabs */}
        <Tabs defaultValue="details">
          <TabsList>
            <TabsTrigger value="details" className="gap-2">
              <Key className="h-4 w-4" />
              Details
            </TabsTrigger>
            <TabsTrigger value="rotation" className="gap-2">
              <RefreshCw className="h-4 w-4" />
              Rotation
            </TabsTrigger>
            <TabsTrigger value="versions" className="gap-2">
              <Clock className="h-4 w-4" />
              Versions
            </TabsTrigger>
          </TabsList>

          {/* ─── Details Tab ─── */}
          <TabsContent value="details" className="space-y-6 mt-4">
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
          </TabsContent>

          {/* ─── Rotation Tab ─── */}
          <TabsContent value="rotation" className="space-y-6 mt-4">
            {rotationLoading ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <>
                {/* Rotation Status Card */}
                <Card>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="text-base flex items-center gap-2">
                          <RefreshCw className="h-4 w-4" />
                          Rotation Status
                        </CardTitle>
                        <CardDescription>
                          {rotationConfig
                            ? "Automatic rotation is configured for this secret"
                            : "No rotation configured — set up automatic rotation below"}
                        </CardDescription>
                      </div>
                      {rotationConfig && (
                        <Badge
                          variant="outline"
                          className={
                            rotationConfig.enabled
                              ? "text-green-400 border-green-500/30"
                              : "text-muted-foreground"
                          }
                        >
                          {rotationConfig.enabled ? (
                            <>
                              <Power className="h-3 w-3 mr-1" />
                              Enabled
                            </>
                          ) : (
                            <>
                              <PowerOff className="h-3 w-3 mr-1" />
                              Disabled
                            </>
                          )}
                        </Badge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent>
                    {rotationConfig ? (
                      <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
                        <div>
                          <dt className="text-muted-foreground mb-1 flex items-center gap-1">
                            <Calendar className="h-3 w-3" />
                            Schedule (Cron)
                          </dt>
                          <dd className="font-mono bg-muted/50 rounded px-2 py-1 inline-block">
                            {rotationConfig.schedule}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground mb-1 flex items-center gap-1">
                            <Settings className="h-3 w-3" />
                            Connector Type
                          </dt>
                          <dd>
                            <Badge variant="secondary" className="font-mono text-xs">
                              {rotationConfig.connector_type}
                            </Badge>
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground mb-1">Last Rotated</dt>
                          <dd>
                            {rotationConfig.last_rotated_at
                              ? format(new Date(rotationConfig.last_rotated_at), "PPpp")
                              : "Never"}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground mb-1">Next Rotation</dt>
                          <dd>
                            {rotationConfig.next_rotation_at
                              ? format(new Date(rotationConfig.next_rotation_at), "PPpp")
                              : "Not scheduled"}
                          </dd>
                        </div>
                      </dl>
                    ) : (
                      <p className="text-sm text-muted-foreground">
                        Click &quot;Set Rotation&quot; to configure automatic secret rotation.
                      </p>
                    )}
                  </CardContent>
                </Card>

                {/* Rotation Actions */}
                <div className="flex gap-3">
                  <Dialog open={rotationDialogOpen} onOpenChange={setRotationDialogOpen}>
                    <DialogTrigger asChild>
                      <Button variant="outline">
                        <Settings className="mr-2 h-4 w-4" />
                        {rotationConfig ? "Edit Rotation" : "Set Rotation"}
                      </Button>
                    </DialogTrigger>
                    <DialogContent>
                      <form onSubmit={handleSetRotation}>
                        <DialogHeader>
                          <DialogTitle>
                            {rotationConfig ? "Edit Rotation Config" : "Set Rotation"}
                          </DialogTitle>
                          <DialogDescription>
                            Configure automatic rotation for this secret
                          </DialogDescription>
                        </DialogHeader>
                        <div className="space-y-4 py-4">
                          <div className="space-y-2">
                            <Label htmlFor="cron-schedule">Cron Schedule</Label>
                            <Input
                              id="cron-schedule"
                              placeholder="0 0 * * * (daily at midnight)"
                              value={rotationForm.schedule}
                              onChange={(e) =>
                                setRotationForm((f) => ({
                                  ...f,
                                  schedule: e.target.value,
                                }))
                              }
                              required
                              className="font-mono"
                            />
                            <p className="text-xs text-muted-foreground">
                              Standard cron format: min hour day month weekday
                            </p>
                          </div>
                          <div className="space-y-2">
                            <Label>Connector Type</Label>
                            <Select
                              value={rotationForm.connector_type}
                              onValueChange={(v) =>
                                setRotationForm((f) => ({
                                  ...f,
                                  connector_type: v as ConnectorType,
                                }))
                              }
                            >
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="random_password">
                                  Random Password
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="flex items-center justify-between rounded-md border p-3">
                            <div>
                              <Label className="text-sm font-medium">Enabled</Label>
                              <p className="text-xs text-muted-foreground mt-0.5">
                                Toggle automatic rotation on/off
                              </p>
                            </div>
                            <Button
                              type="button"
                              variant={rotationForm.enabled ? "default" : "outline"}
                              size="sm"
                              onClick={() =>
                                setRotationForm((f) => ({
                                  ...f,
                                  enabled: !f.enabled,
                                }))
                              }
                            >
                              {rotationForm.enabled ? "Enabled" : "Disabled"}
                            </Button>
                          </div>
                        </div>
                        <DialogFooter>
                          <Button
                            type="button"
                            variant="outline"
                            onClick={() => setRotationDialogOpen(false)}
                          >
                            Cancel
                          </Button>
                          <Button type="submit" disabled={settingRotation}>
                            {settingRotation ? (
                              <>
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                Saving…
                              </>
                            ) : (
                              "Save Rotation"
                            )}
                          </Button>
                        </DialogFooter>
                      </form>
                    </DialogContent>
                  </Dialog>

                  <Button
                    variant="default"
                    onClick={handleRotateNow}
                    disabled={rotating}
                  >
                    {rotating ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        Rotating…
                      </>
                    ) : (
                      <>
                        <Play className="mr-2 h-4 w-4" />
                        Rotate Now
                      </>
                    )}
                  </Button>
                </div>
              </>
            )}
          </TabsContent>

          {/* ─── Versions Tab ─── */}
          <TabsContent value="versions" className="space-y-6 mt-4">
            {/* Compare Versions */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <GitCompare className="h-4 w-4" />
                  Compare Versions
                </CardTitle>
                <CardDescription>
                  Select two versions to compare side-by-side
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-end gap-3 flex-wrap">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Version A</Label>
                    <Select
                      value={diffVersionA === "" ? "" : String(diffVersionA)}
                      onValueChange={(v) => setDiffVersionA(v ? parseInt(v) : "")}
                    >
                      <SelectTrigger className="w-[140px]">
                        <SelectValue placeholder="Select…" />
                      </SelectTrigger>
                      <SelectContent>
                        {sortedVersions.map((v) => (
                          <SelectItem key={v.version} value={String(v.version)}>
                            v{v.version}
                            {v.version === secret.version ? " (current)" : ""}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Version B</Label>
                    <Select
                      value={diffVersionB === "" ? "" : String(diffVersionB)}
                      onValueChange={(v) => setDiffVersionB(v ? parseInt(v) : "")}
                    >
                      <SelectTrigger className="w-[140px]">
                        <SelectValue placeholder="Select…" />
                      </SelectTrigger>
                      <SelectContent>
                        {sortedVersions.map((v) => (
                          <SelectItem key={v.version} value={String(v.version)}>
                            v{v.version}
                            {v.version === secret.version ? " (current)" : ""}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <Button
                    variant="outline"
                    onClick={handleCompareVersions}
                    disabled={diffLoading || diffVersionA === "" || diffVersionB === ""}
                  >
                    {diffLoading ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <GitCompare className="mr-2 h-4 w-4" />
                    )}
                    Compare
                  </Button>
                </div>

                {/* Diff Display */}
                {diffValueA !== null && diffValueB !== null && (
                  <div className="mt-4 space-y-3">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-medium">Comparison Result</p>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDiffRevealed(!diffRevealed)}
                        className="gap-1"
                      >
                        {diffRevealed ? (
                          <>
                            <EyeOff className="h-3.5 w-3.5" />
                            Mask
                          </>
                        ) : (
                          <>
                            <Eye className="h-3.5 w-3.5" />
                            Reveal
                          </>
                        )}
                      </Button>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                      <div>
                        <p className="text-xs text-muted-foreground mb-1.5 font-medium">
                          Version {diffVersionA}
                        </p>
                        <div className="bg-muted/50 border rounded-md px-4 py-3 font-mono text-sm overflow-x-auto min-h-[60px]">
                          {diffRevealed ? (
                            <span className="break-all select-all whitespace-pre-wrap">
                              {diffValueA}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">{maskedValue}</span>
                          )}
                        </div>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground mb-1.5 font-medium">
                          Version {diffVersionB}
                        </p>
                        <div className="bg-muted/50 border rounded-md px-4 py-3 font-mono text-sm overflow-x-auto min-h-[60px]">
                          {diffRevealed ? (
                            <span className="break-all select-all whitespace-pre-wrap">
                              {diffValueB}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">{maskedValue}</span>
                          )}
                        </div>
                      </div>
                    </div>
                    {diffRevealed && diffValueA !== diffValueB && (
                      <Badge variant="secondary" className="text-yellow-400 bg-yellow-500/10 border-yellow-500/20">
                        Values differ
                      </Badge>
                    )}
                    {diffRevealed && diffValueA === diffValueB && (
                      <Badge variant="secondary" className="text-green-400 bg-green-500/10 border-green-500/20">
                        Values are identical
                      </Badge>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Version History Table */}
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
                      {sortedVersions.map((v) => (
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
          </TabsContent>
        </Tabs>
      </div>
    </AppShell>
  );
}
