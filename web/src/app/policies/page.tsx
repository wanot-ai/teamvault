"use client";

import { useEffect, useState, useCallback } from "react";
import {
  iamPolicies as policiesApi,
  type IAMPolicy,
  type IAMPolicyType,
  type CreateIAMPolicyRequest,
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
import { Textarea } from "@/components/ui/textarea";
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
  ShieldCheck,
  Plus,
  Loader2,
  FileCode2,
  Trash2,
  Eye,
  X,
  ToggleLeft,
  ToggleRight,
  Shield,
  UserCheck,
  GitBranch,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

const POLICY_TYPE_META: Record<
  IAMPolicyType,
  { label: string; description: string; icon: React.ElementType; color: string }
> = {
  rbac: {
    label: "RBAC",
    description: "Role-Based Access Control — assign permissions via roles",
    icon: Shield,
    color: "text-blue-400",
  },
  abac: {
    label: "ABAC",
    description: "Attribute-Based Access Control — conditions on user/resource attributes",
    icon: UserCheck,
    color: "text-green-400",
  },
  pbac: {
    label: "PBAC",
    description: "Policy-Based Access Control — declarative HCL policy rules",
    icon: GitBranch,
    color: "text-purple-400",
  },
};

const HCL_TEMPLATES: Record<IAMPolicyType, string> = {
  rbac: `policy "example-rbac" {
  type = "rbac"
  effect = "allow"

  role "admin" {
    actions = ["secrets:read", "secrets:write", "secrets:delete"]
    resources = ["projects/*"]
  }

  role "viewer" {
    actions = ["secrets:read"]
    resources = ["projects/*"]
  }
}`,
  abac: `policy "example-abac" {
  type = "abac"
  effect = "allow"

  rule {
    actions   = ["secrets:read"]
    resources = ["projects/\${project.id}/*"]

    condition {
      attribute = "user.department"
      operator  = "equals"
      value     = "engineering"
    }

    condition {
      attribute = "resource.classification"
      operator  = "not_equals"
      value     = "top-secret"
    }
  }
}`,
  pbac: `policy "example-pbac" {
  type = "pbac"
  effect = "allow"

  rule {
    actions   = ["secrets:read", "secrets:write"]
    resources = ["projects/production/*"]
    subjects  = ["team:backend-engineering"]

    condition {
      time_window {
        start = "09:00"
        end   = "18:00"
        tz    = "UTC"
      }
    }

    condition {
      ip_range = ["10.0.0.0/8", "172.16.0.0/12"]
    }
  }
}`,
};

export default function IAMPoliciesPage() {
  const [allPolicies, setAllPolicies] = useState<IAMPolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<IAMPolicyType>("rbac");

  // Create dialog
  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createMode, setCreateMode] = useState<"hcl" | "form">("form");
  const [form, setForm] = useState<CreateIAMPolicyRequest>({
    name: "",
    description: "",
    type: "rbac",
    effect: "allow",
    hcl_source: "",
    rules: [],
    enabled: true,
  });

  // Detail view
  const [selectedPolicy, setSelectedPolicy] = useState<IAMPolicy | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  // Deleting/toggling
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  // Form-based rule state
  const [ruleActions, setRuleActions] = useState("");
  const [ruleResources, setRuleResources] = useState("");
  const [ruleSubjects, setRuleSubjects] = useState("");

  const loadPolicies = useCallback(async () => {
    setLoading(true);
    try {
      const data = await policiesApi.list();
      setAllPolicies(data || []);
    } catch (err) {
      toast.error("Failed to load IAM policies");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadPolicies();
  }, [loadPolicies]);

  const filteredPolicies = allPolicies.filter((p) => p.type === activeTab);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      const payload = { ...form };
      if (createMode === "form") {
        payload.rules = [
          {
            actions: ruleActions
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
            resources: ruleResources
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
            subjects: ruleSubjects
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
            conditions: null,
          },
        ];
        payload.hcl_source = undefined;
      } else {
        payload.rules = undefined;
      }
      await policiesApi.create(payload);
      toast.success(`Policy "${form.name}" created`);
      setDialogOpen(false);
      resetForm();
      loadPolicies();
    } catch (err) {
      toast.error("Failed to create policy");
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const resetForm = () => {
    setForm({
      name: "",
      description: "",
      type: activeTab,
      effect: "allow",
      hcl_source: "",
      rules: [],
      enabled: true,
    });
    setRuleActions("");
    setRuleResources("");
    setRuleSubjects("");
    setCreateMode("form");
  };

  const handleDelete = async (policyId: string) => {
    setDeletingId(policyId);
    try {
      await policiesApi.delete(policyId);
      toast.success("Policy deleted");
      loadPolicies();
    } catch (err) {
      toast.error("Failed to delete policy");
      console.error(err);
    } finally {
      setDeletingId(null);
    }
  };

  const handleToggle = async (policy: IAMPolicy) => {
    setTogglingId(policy.id);
    try {
      await policiesApi.toggle(policy.id, !policy.enabled);
      toast.success(`Policy ${policy.enabled ? "disabled" : "enabled"}`);
      loadPolicies();
    } catch (err) {
      toast.error("Failed to toggle policy");
      console.error(err);
    } finally {
      setTogglingId(null);
    }
  };

  const handleViewDetail = (policy: IAMPolicy) => {
    setSelectedPolicy(policy);
    setDetailOpen(true);
  };

  const openCreateWithType = (type: IAMPolicyType) => {
    setForm((f) => ({ ...f, type }));
    setDialogOpen(true);
  };

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
              <ShieldCheck className="h-6 w-6 text-primary" />
              IAM Policies
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage access control policies for your organization
            </p>
          </div>
          <Dialog open={dialogOpen} onOpenChange={(open) => {
            if (!open) resetForm();
            setDialogOpen(open);
          }}>
            <DialogTrigger asChild>
              <Button onClick={() => openCreateWithType(activeTab)}>
                <Plus className="mr-2 h-4 w-4" />
                New Policy
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
              <form onSubmit={handleCreate}>
                <DialogHeader>
                  <DialogTitle>Create IAM Policy</DialogTitle>
                  <DialogDescription>
                    Define access control rules using HCL or the form builder
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  {/* Name & Description */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label htmlFor="policy-name">Name</Label>
                      <Input
                        id="policy-name"
                        placeholder="backend-read-access"
                        value={form.name}
                        onChange={(e) =>
                          setForm((f) => ({ ...f, name: e.target.value }))
                        }
                        required
                        autoFocus
                        className="font-mono"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Type</Label>
                      <Select
                        value={form.type}
                        onValueChange={(v) =>
                          setForm((f) => ({
                            ...f,
                            type: v as IAMPolicyType,
                          }))
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="rbac">RBAC — Role-Based</SelectItem>
                          <SelectItem value="abac">ABAC — Attribute-Based</SelectItem>
                          <SelectItem value="pbac">PBAC — Policy-Based</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="policy-desc">Description</Label>
                    <Input
                      id="policy-desc"
                      placeholder="Allow backend team to read production secrets"
                      value={form.description}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, description: e.target.value }))
                      }
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>Effect</Label>
                    <Select
                      value={form.effect}
                      onValueChange={(v) =>
                        setForm((f) => ({
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

                  {/* Mode toggle */}
                  <div className="flex items-center gap-2 pt-2">
                    <Button
                      type="button"
                      variant={createMode === "form" ? "secondary" : "ghost"}
                      size="sm"
                      onClick={() => setCreateMode("form")}
                    >
                      Form Builder
                    </Button>
                    <Button
                      type="button"
                      variant={createMode === "hcl" ? "secondary" : "ghost"}
                      size="sm"
                      onClick={() => {
                        setCreateMode("hcl");
                        if (!form.hcl_source) {
                          setForm((f) => ({
                            ...f,
                            hcl_source: HCL_TEMPLATES[f.type],
                          }));
                        }
                      }}
                    >
                      <FileCode2 className="mr-1.5 h-3.5 w-3.5" />
                      HCL Editor
                    </Button>
                  </div>

                  {createMode === "hcl" ? (
                    <div className="space-y-2">
                      <Label>HCL Policy Source</Label>
                      <Textarea
                        value={form.hcl_source}
                        onChange={(e) =>
                          setForm((f) => ({
                            ...f,
                            hcl_source: e.target.value,
                          }))
                        }
                        rows={16}
                        className="font-mono text-sm bg-muted/50 leading-relaxed"
                        placeholder="Enter HCL policy definition…"
                      />
                      <p className="text-xs text-muted-foreground">
                        Use HashiCorp HCL syntax to define policy rules
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-4 border rounded-md p-4 bg-muted/20">
                      <p className="text-sm font-medium">Rule Definition</p>
                      <div className="space-y-2">
                        <Label htmlFor="rule-actions">Actions</Label>
                        <Input
                          id="rule-actions"
                          placeholder="secrets:read, secrets:write"
                          value={ruleActions}
                          onChange={(e) => setRuleActions(e.target.value)}
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          Comma-separated: secrets:read, secrets:write, secrets:delete
                        </p>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="rule-resources">Resources</Label>
                        <Input
                          id="rule-resources"
                          placeholder="projects/production/*"
                          value={ruleResources}
                          onChange={(e) => setRuleResources(e.target.value)}
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          Comma-separated resource patterns with * wildcards
                        </p>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="rule-subjects">Subjects</Label>
                        <Input
                          id="rule-subjects"
                          placeholder="team:backend, user:alice@example.com"
                          value={ruleSubjects}
                          onChange={(e) => setRuleSubjects(e.target.value)}
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          Comma-separated: team:name, user:email, role:admin
                        </p>
                      </div>
                    </div>
                  )}
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
                      "Create Policy"
                    )}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {/* Type Tabs */}
        <Tabs
          value={activeTab}
          onValueChange={(v) => setActiveTab(v as IAMPolicyType)}
        >
          <TabsList>
            {(["rbac", "abac", "pbac"] as IAMPolicyType[]).map((type) => {
              const meta = POLICY_TYPE_META[type];
              const count = allPolicies.filter((p) => p.type === type).length;
              return (
                <TabsTrigger key={type} value={type} className="gap-2">
                  <meta.icon className={`h-4 w-4 ${meta.color}`} />
                  {meta.label}
                  {count > 0 && (
                    <Badge
                      variant="secondary"
                      className="ml-1 h-5 px-1.5 text-xs"
                    >
                      {count}
                    </Badge>
                  )}
                </TabsTrigger>
              );
            })}
          </TabsList>

          {(["rbac", "abac", "pbac"] as IAMPolicyType[]).map((type) => {
            const meta = POLICY_TYPE_META[type];
            const typePolicies = allPolicies.filter((p) => p.type === type);
            return (
              <TabsContent key={type} value={type} className="space-y-4 mt-4">
                {/* Type description card */}
                <Card className="bg-muted/20">
                  <CardContent className="pt-4 pb-4">
                    <div className="flex items-center gap-3">
                      <meta.icon className={`h-5 w-5 ${meta.color}`} />
                      <div>
                        <p className="text-sm font-medium">{meta.label}</p>
                        <p className="text-xs text-muted-foreground">
                          {meta.description}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                {loading ? (
                  <div className="flex items-center justify-center py-16">
                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                  </div>
                ) : typePolicies.length === 0 ? (
                  <Card className="border-dashed">
                    <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                      <meta.icon
                        className={`h-12 w-12 ${meta.color} opacity-50 mb-4`}
                      />
                      <h3 className="text-lg font-medium mb-1">
                        No {meta.label} policies
                      </h3>
                      <p className="text-sm text-muted-foreground mb-4">
                        Create your first {meta.label} policy to define access rules
                      </p>
                      <Button onClick={() => openCreateWithType(type)}>
                        <Plus className="mr-2 h-4 w-4" />
                        New {meta.label} Policy
                      </Button>
                    </CardContent>
                  </Card>
                ) : (
                  <div className="border rounded-lg">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Name</TableHead>
                          <TableHead>Effect</TableHead>
                          <TableHead>Rules</TableHead>
                          <TableHead>Status</TableHead>
                          <TableHead>Created</TableHead>
                          <TableHead className="w-[120px]" />
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {typePolicies.map((policy) => (
                          <TableRow key={policy.id}>
                            <TableCell>
                              <div>
                                <p className="font-medium">{policy.name}</p>
                                {policy.description && (
                                  <p className="text-xs text-muted-foreground line-clamp-1">
                                    {policy.description}
                                  </p>
                                )}
                              </div>
                            </TableCell>
                            <TableCell>
                              <Badge
                                variant={
                                  policy.effect === "allow"
                                    ? "secondary"
                                    : "destructive"
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
                              <Badge
                                variant="outline"
                                className="font-mono text-xs"
                              >
                                {policy.rules?.length ?? 0} rule
                                {(policy.rules?.length ?? 0) !== 1 && "s"}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <Badge
                                variant="outline"
                                className={
                                  policy.enabled
                                    ? "text-green-400 border-green-500/30"
                                    : "text-muted-foreground"
                                }
                              >
                                {policy.enabled ? "Enabled" : "Disabled"}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground">
                              {formatDistanceToNow(
                                new Date(policy.created_at),
                                { addSuffix: true }
                              )}
                            </TableCell>
                            <TableCell>
                              <div className="flex gap-1">
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8"
                                  onClick={() => handleViewDetail(policy)}
                                  title="View details"
                                >
                                  <Eye className="h-4 w-4" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8"
                                  onClick={() => handleToggle(policy)}
                                  disabled={togglingId === policy.id}
                                  title={
                                    policy.enabled ? "Disable" : "Enable"
                                  }
                                >
                                  {togglingId === policy.id ? (
                                    <Loader2 className="h-4 w-4 animate-spin" />
                                  ) : policy.enabled ? (
                                    <ToggleRight className="h-4 w-4 text-green-400" />
                                  ) : (
                                    <ToggleLeft className="h-4 w-4" />
                                  )}
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8 text-muted-foreground hover:text-destructive"
                                  onClick={() => handleDelete(policy.id)}
                                  disabled={deletingId === policy.id}
                                >
                                  {deletingId === policy.id ? (
                                    <Loader2 className="h-4 w-4 animate-spin" />
                                  ) : (
                                    <Trash2 className="h-4 w-4" />
                                  )}
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </TabsContent>
            );
          })}
        </Tabs>

        {/* Policy Detail Modal */}
        <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
          <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
            {selectedPolicy && (
              <>
                <DialogHeader>
                  <DialogTitle className="flex items-center gap-2">
                    <FileCode2 className="h-5 w-5 text-primary" />
                    {selectedPolicy.name}
                  </DialogTitle>
                  <DialogDescription>
                    {selectedPolicy.description ||
                      `${POLICY_TYPE_META[selectedPolicy.type].label} policy`}
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-6 py-4">
                  {/* Metadata */}
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">Type</p>
                      <Badge variant="secondary">
                        {POLICY_TYPE_META[selectedPolicy.type].label}
                      </Badge>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">
                        Effect
                      </p>
                      <Badge
                        variant={
                          selectedPolicy.effect === "allow"
                            ? "secondary"
                            : "destructive"
                        }
                        className={
                          selectedPolicy.effect === "allow"
                            ? "text-green-400 bg-green-500/10"
                            : ""
                        }
                      >
                        {selectedPolicy.effect}
                      </Badge>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">
                        Status
                      </p>
                      <Badge
                        variant="outline"
                        className={
                          selectedPolicy.enabled
                            ? "text-green-400 border-green-500/30"
                            : "text-muted-foreground"
                        }
                      >
                        {selectedPolicy.enabled ? "Enabled" : "Disabled"}
                      </Badge>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">
                        Created
                      </p>
                      <p className="text-sm">
                        {formatDistanceToNow(
                          new Date(selectedPolicy.created_at),
                          { addSuffix: true }
                        )}
                      </p>
                    </div>
                  </div>

                  {/* HCL Source */}
                  {selectedPolicy.hcl_source && (
                    <div>
                      <p className="text-sm font-medium mb-2 flex items-center gap-2">
                        <FileCode2 className="h-4 w-4" />
                        HCL Source
                      </p>
                      <div className="bg-muted/50 border rounded-lg p-4 overflow-x-auto">
                        <pre className="font-mono text-sm text-foreground whitespace-pre leading-relaxed">
                          {selectedPolicy.hcl_source}
                        </pre>
                      </div>
                    </div>
                  )}

                  {/* Parsed Rules */}
                  {selectedPolicy.rules && selectedPolicy.rules.length > 0 && (
                    <div>
                      <p className="text-sm font-medium mb-2">
                        Parsed Rules ({selectedPolicy.rules.length})
                      </p>
                      <div className="space-y-3">
                        {selectedPolicy.rules.map((rule, idx) => (
                          <Card key={rule.id || idx} className="bg-muted/20">
                            <CardContent className="pt-4 pb-4">
                              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                                <div>
                                  <p className="text-xs text-muted-foreground mb-1.5">
                                    Actions
                                  </p>
                                  <div className="flex flex-wrap gap-1">
                                    {rule.actions.map((a) => (
                                      <Badge
                                        key={a}
                                        variant="outline"
                                        className="font-mono text-xs"
                                      >
                                        {a}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                                <div>
                                  <p className="text-xs text-muted-foreground mb-1.5">
                                    Resources
                                  </p>
                                  <div className="flex flex-wrap gap-1">
                                    {rule.resources.map((r) => (
                                      <Badge
                                        key={r}
                                        variant="secondary"
                                        className="font-mono text-xs"
                                      >
                                        {r}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                                <div>
                                  <p className="text-xs text-muted-foreground mb-1.5">
                                    Subjects
                                  </p>
                                  <div className="flex flex-wrap gap-1">
                                    {rule.subjects.map((s) => (
                                      <Badge
                                        key={s}
                                        variant="secondary"
                                        className="font-mono text-xs"
                                      >
                                        {s}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                              </div>
                              {rule.conditions &&
                                Object.keys(rule.conditions).length > 0 && (
                                  <div className="mt-3 pt-3 border-t">
                                    <p className="text-xs text-muted-foreground mb-1.5">
                                      Conditions
                                    </p>
                                    <pre className="font-mono text-xs bg-muted/50 rounded p-2 overflow-x-auto">
                                      {JSON.stringify(
                                        rule.conditions,
                                        null,
                                        2
                                      )}
                                    </pre>
                                  </div>
                                )}
                            </CardContent>
                          </Card>
                        ))}
                      </div>
                    </div>
                  )}
                </div>

                <DialogFooter>
                  <Button
                    variant="outline"
                    onClick={() => setDetailOpen(false)}
                  >
                    Close
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
      </div>
    </AppShell>
  );
}
