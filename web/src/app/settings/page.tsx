"use client";

import { useEffect, useState } from "react";
import {
  serviceAccounts as saApi,
  policies as policiesApi,
  projects as projectsApi,
  type ServiceAccount,
  type Policy,
  type Project,
  type CreateServiceAccountRequest,
  type CreatePolicyRequest,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Settings,
  Plus,
  Loader2,
  Bot,
  ShieldCheck,
  Copy,
  Check,
  Eye,
  EyeOff,
  AlertTriangle,
} from "lucide-react";
import { toast } from "sonner";
import { format, formatDistanceToNow } from "date-fns";
import { useCopyToClipboard } from "@/lib/hooks";

export default function SettingsPage() {
  const [saList, setSaList] = useState<ServiceAccount[]>([]);
  const [policyList, setPolicyList] = useState<Policy[]>([]);
  const [projectList, setProjectList] = useState<Project[]>([]);
  const [loadingSA, setLoadingSA] = useState(true);
  const [loadingPolicies, setLoadingPolicies] = useState(true);

  // SA creation
  const [saDialogOpen, setSaDialogOpen] = useState(false);
  const [creatingSA, setCreatingSA] = useState(false);
  const [saForm, setSaForm] = useState<CreateServiceAccountRequest>({
    name: "",
    project_id: "",
    scopes: ["read"],
  });
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [tokenRevealed, setTokenRevealed] = useState(false);
  const { copied: tokenCopied, copy: copyToken } = useCopyToClipboard();

  // Policy creation
  const [policyDialogOpen, setPolicyDialogOpen] = useState(false);
  const [creatingPolicy, setCreatingPolicy] = useState(false);
  const [policyForm, setPolicyForm] = useState<CreatePolicyRequest>({
    name: "",
    effect: "allow",
    actions: ["read"],
    resource_pattern: "",
    subject_type: "service_account",
  });

  const loadSAs = async () => {
    try {
      const data = await saApi.list();
      setSaList(data || []);
    } catch (err) {
      toast.error("Failed to load service accounts");
      console.error(err);
    } finally {
      setLoadingSA(false);
    }
  };

  const loadPolicies = async () => {
    try {
      const data = await policiesApi.list();
      setPolicyList(data || []);
    } catch (err) {
      toast.error("Failed to load policies");
      console.error(err);
    } finally {
      setLoadingPolicies(false);
    }
  };

  const loadProjects = async () => {
    try {
      const data = await projectsApi.list();
      setProjectList(data || []);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    loadSAs();
    loadPolicies();
    loadProjects();
  }, []);

  const handleCreateSA = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingSA(true);
    try {
      const result = await saApi.create(saForm);
      if (result.token) {
        setCreatedToken(result.token);
      } else {
        toast.success(`Service account "${saForm.name}" created`);
        closeSADialog();
      }
      loadSAs();
    } catch (err) {
      toast.error("Failed to create service account");
      console.error(err);
    } finally {
      setCreatingSA(false);
    }
  };

  const closeSADialog = () => {
    setSaDialogOpen(false);
    setSaForm({ name: "", project_id: "", scopes: ["read"] });
    setCreatedToken(null);
    setTokenRevealed(false);
  };

  const handleCreatePolicy = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingPolicy(true);
    try {
      await policiesApi.create(policyForm);
      toast.success(`Policy "${policyForm.name}" created`);
      setPolicyDialogOpen(false);
      setPolicyForm({
        name: "",
        effect: "allow",
        actions: ["read"],
        resource_pattern: "",
        subject_type: "service_account",
      });
      loadPolicies();
    } catch (err) {
      toast.error("Failed to create policy");
      console.error(err);
    } finally {
      setCreatingPolicy(false);
    }
  };

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
            <Settings className="h-6 w-6 text-primary" />
            Settings
          </h1>
          <p className="text-muted-foreground mt-1">
            Manage service accounts and access policies
          </p>
        </div>

        <Tabs defaultValue="service-accounts">
          <TabsList>
            <TabsTrigger value="service-accounts" className="gap-2">
              <Bot className="h-4 w-4" />
              Service Accounts
            </TabsTrigger>
            <TabsTrigger value="policies" className="gap-2">
              <ShieldCheck className="h-4 w-4" />
              Policies
            </TabsTrigger>
          </TabsList>

          {/* ─── SERVICE ACCOUNTS TAB ─── */}
          <TabsContent value="service-accounts" className="space-y-4 mt-4">
            <div className="flex justify-end">
              <Dialog open={saDialogOpen} onOpenChange={(open) => {
                if (!open) closeSADialog();
                else setSaDialogOpen(true);
              }}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    New Service Account
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  {createdToken ? (
                    <>
                      <DialogHeader>
                        <DialogTitle>Token Created</DialogTitle>
                        <DialogDescription>
                          Copy this token now — you won&apos;t be able to see it again.
                        </DialogDescription>
                      </DialogHeader>
                      <div className="py-4">
                        <div className="rounded-md bg-yellow-500/10 border border-yellow-500/20 px-4 py-3 mb-4 flex items-start gap-2">
                          <AlertTriangle className="h-4 w-4 text-yellow-500 mt-0.5 flex-shrink-0" />
                          <p className="text-sm text-yellow-200">
                            This token is shown only once. Store it securely.
                          </p>
                        </div>
                        <Label className="text-xs text-muted-foreground">Service Account Token</Label>
                        <div className="flex items-center gap-2 mt-2">
                          <div className="flex-1 bg-muted/50 border rounded-md px-3 py-2 font-mono text-xs overflow-x-auto">
                            {tokenRevealed ? (
                              <span className="break-all select-all">{createdToken}</span>
                            ) : (
                              <span className="text-muted-foreground">
                                sa.••••••••••••••••••••••••••••••••
                              </span>
                            )}
                          </div>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-9 w-9"
                            onClick={() => setTokenRevealed(!tokenRevealed)}
                          >
                            {tokenRevealed ? (
                              <EyeOff className="h-4 w-4" />
                            ) : (
                              <Eye className="h-4 w-4" />
                            )}
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-9 w-9"
                            onClick={() => {
                              copyToken(createdToken);
                              toast.success("Token copied to clipboard");
                            }}
                          >
                            {tokenCopied ? (
                              <Check className="h-4 w-4 text-green-500" />
                            ) : (
                              <Copy className="h-4 w-4" />
                            )}
                          </Button>
                        </div>
                      </div>
                      <DialogFooter>
                        <Button onClick={closeSADialog}>Done</Button>
                      </DialogFooter>
                    </>
                  ) : (
                    <form onSubmit={handleCreateSA}>
                      <DialogHeader>
                        <DialogTitle>Create Service Account</DialogTitle>
                        <DialogDescription>
                          Service accounts provide programmatic access to secrets
                        </DialogDescription>
                      </DialogHeader>
                      <div className="space-y-4 py-4">
                        <div className="space-y-2">
                          <Label htmlFor="sa-name">Name</Label>
                          <Input
                            id="sa-name"
                            placeholder="ci-deploy"
                            value={saForm.name}
                            onChange={(e) =>
                              setSaForm((f) => ({ ...f, name: e.target.value }))
                            }
                            required
                            autoFocus
                          />
                        </div>
                        <div className="space-y-2">
                          <Label>Project</Label>
                          <Select
                            value={saForm.project_id}
                            onValueChange={(v) =>
                              setSaForm((f) => ({ ...f, project_id: v }))
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
                          <Label>Scopes</Label>
                          <Select
                            value={saForm.scopes[0]}
                            onValueChange={(v) =>
                              setSaForm((f) => ({ ...f, scopes: [v] }))
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="read">Read</SelectItem>
                              <SelectItem value="write">Write</SelectItem>
                              <SelectItem value="read,write">Read & Write</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                      <DialogFooter>
                        <Button
                          type="button"
                          variant="outline"
                          onClick={closeSADialog}
                        >
                          Cancel
                        </Button>
                        <Button type="submit" disabled={creatingSA}>
                          {creatingSA ? (
                            <>
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                              Creating…
                            </>
                          ) : (
                            "Create"
                          )}
                        </Button>
                      </DialogFooter>
                    </form>
                  )}
                </DialogContent>
              </Dialog>
            </div>

            {loadingSA ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : saList.length === 0 ? (
              <Card className="border-dashed">
                <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                  <Bot className="h-12 w-12 text-muted-foreground/50 mb-4" />
                  <h3 className="text-lg font-medium mb-1">No service accounts</h3>
                  <p className="text-sm text-muted-foreground">
                    Create a service account for CI/CD or agent access
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="border rounded-lg">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Project</TableHead>
                      <TableHead>Scopes</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Expires</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {saList.map((sa) => {
                      const proj = projectList.find((p) => p.id === sa.project_id);
                      return (
                        <TableRow key={sa.id}>
                          <TableCell className="font-medium">{sa.name}</TableCell>
                          <TableCell className="text-sm">
                            {proj?.name || sa.project_id}
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1">
                              {sa.scopes.map((scope) => (
                                <Badge key={scope} variant="secondary" className="text-xs">
                                  {scope}
                                </Badge>
                              ))}
                            </div>
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {formatDistanceToNow(new Date(sa.created_at), {
                              addSuffix: true,
                            })}
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {sa.expires_at
                              ? format(new Date(sa.expires_at), "PP")
                              : "Never"}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            )}
          </TabsContent>

          {/* ─── POLICIES TAB ─── */}
          <TabsContent value="policies" className="space-y-4 mt-4">
            <div className="flex justify-end">
              <Dialog open={policyDialogOpen} onOpenChange={setPolicyDialogOpen}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    New Policy
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <form onSubmit={handleCreatePolicy}>
                    <DialogHeader>
                      <DialogTitle>Create Policy</DialogTitle>
                      <DialogDescription>
                        Define access rules for secrets
                      </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                      <div className="space-y-2">
                        <Label htmlFor="policy-name">Name</Label>
                        <Input
                          id="policy-name"
                          placeholder="ci-read-only"
                          value={policyForm.name}
                          onChange={(e) =>
                            setPolicyForm((f) => ({ ...f, name: e.target.value }))
                          }
                          required
                          autoFocus
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Effect</Label>
                        <Select
                          value={policyForm.effect}
                          onValueChange={(v) =>
                            setPolicyForm((f) => ({
                              ...f,
                              effect: v as "allow" | "deny",
                            }))
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="allow">Allow</SelectItem>
                            <SelectItem value="deny">Deny</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>Actions</Label>
                        <Select
                          value={policyForm.actions[0]}
                          onValueChange={(v) =>
                            setPolicyForm((f) => ({ ...f, actions: [v] }))
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="read">Read</SelectItem>
                            <SelectItem value="write">Write</SelectItem>
                            <SelectItem value="delete">Delete</SelectItem>
                            <SelectItem value="*">All (*)</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="resource-pattern">Resource Pattern</Label>
                        <Input
                          id="resource-pattern"
                          placeholder="my-project/api-keys/*"
                          value={policyForm.resource_pattern}
                          onChange={(e) =>
                            setPolicyForm((f) => ({
                              ...f,
                              resource_pattern: e.target.value,
                            }))
                          }
                          required
                          className="font-mono"
                        />
                        <p className="text-xs text-muted-foreground">
                          Use * for wildcards: project/path/*
                        </p>
                      </div>
                      <div className="space-y-2">
                        <Label>Subject Type</Label>
                        <Select
                          value={policyForm.subject_type}
                          onValueChange={(v) =>
                            setPolicyForm((f) => ({
                              ...f,
                              subject_type: v as "user" | "service_account",
                            }))
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="user">User</SelectItem>
                            <SelectItem value="service_account">
                              Service Account
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="subject-id">Subject ID (optional)</Label>
                        <Input
                          id="subject-id"
                          placeholder="UUID of user or service account"
                          value={policyForm.subject_id || ""}
                          onChange={(e) =>
                            setPolicyForm((f) => ({
                              ...f,
                              subject_id: e.target.value || undefined,
                            }))
                          }
                          className="font-mono text-xs"
                        />
                        <p className="text-xs text-muted-foreground">
                          Leave empty to apply to all subjects of this type
                        </p>
                      </div>
                    </div>
                    <DialogFooter>
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => setPolicyDialogOpen(false)}
                      >
                        Cancel
                      </Button>
                      <Button type="submit" disabled={creatingPolicy}>
                        {creatingPolicy ? (
                          <>
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            Creating…
                          </>
                        ) : (
                          "Create Policy"
                        )}
                      </Button>
                    </DialogFooter>
                  </form>
                </DialogContent>
              </Dialog>
            </div>

            {loadingPolicies ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : policyList.length === 0 ? (
              <Card className="border-dashed">
                <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                  <ShieldCheck className="h-12 w-12 text-muted-foreground/50 mb-4" />
                  <h3 className="text-lg font-medium mb-1">No policies</h3>
                  <p className="text-sm text-muted-foreground">
                    Policies control who can access what secrets
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="border rounded-lg">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Effect</TableHead>
                      <TableHead>Actions</TableHead>
                      <TableHead>Resource</TableHead>
                      <TableHead>Subject</TableHead>
                      <TableHead>Created</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {policyList.map((policy) => (
                      <TableRow key={policy.id}>
                        <TableCell className="font-medium">
                          {policy.name}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              policy.effect === "allow" ? "secondary" : "destructive"
                            }
                            className={
                              policy.effect === "allow"
                                ? "text-green-400 bg-green-500/10"
                                : ""
                            }
                          >
                            {policy.effect}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            {policy.actions.map((action) => (
                              <Badge key={action} variant="outline" className="font-mono text-xs">
                                {action}
                              </Badge>
                            ))}
                          </div>
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {policy.resource_pattern}
                        </TableCell>
                        <TableCell className="text-sm">
                          <span className="capitalize">{policy.subject_type.replace("_", " ")}</span>
                          {policy.subject_id && (
                            <span className="block font-mono text-xs text-muted-foreground truncate max-w-[100px]">
                              {policy.subject_id}
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {formatDistanceToNow(new Date(policy.created_at), {
                            addSuffix: true,
                          })}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </AppShell>
  );
}
