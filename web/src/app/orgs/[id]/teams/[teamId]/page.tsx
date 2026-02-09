"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  orgs as orgsApi,
  teams as teamsApi,
  teamMembers as membersApi,
  agents as agentsApi,
  type Organization,
  type Team,
  type TeamMember,
  type Agent,
  type AddTeamMemberRequest,
  type CreateAgentRequest,
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
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  ChevronLeft,
  Plus,
  Loader2,
  Users,
  Bot,
  UsersRound,
  Trash2,
  Copy,
  Check,
  Eye,
  EyeOff,
  AlertTriangle,
  Shield,
  Clock,
} from "lucide-react";
import { toast } from "sonner";
import { format, formatDistanceToNow } from "date-fns";
import { useCopyToClipboard } from "@/lib/hooks";

const AVAILABLE_SCOPES = [
  { id: "secrets:read", label: "Read Secrets", description: "Read secret values" },
  { id: "secrets:write", label: "Write Secrets", description: "Create and update secrets" },
  { id: "secrets:delete", label: "Delete Secrets", description: "Delete secrets" },
  { id: "projects:read", label: "Read Projects", description: "List and view projects" },
  { id: "projects:write", label: "Write Projects", description: "Create and update projects" },
  { id: "audit:read", label: "Read Audit", description: "View audit logs" },
  { id: "policies:read", label: "Read Policies", description: "View IAM policies" },
  { id: "policies:write", label: "Write Policies", description: "Manage IAM policies" },
];

const TTL_OPTIONS = [
  { value: "0", label: "No expiry" },
  { value: "1", label: "1 hour" },
  { value: "24", label: "1 day" },
  { value: "168", label: "7 days" },
  { value: "720", label: "30 days" },
  { value: "2160", label: "90 days" },
  { value: "8760", label: "1 year" },
];

function TokenStatusBadge({ status }: { status: Agent["token_status"] }) {
  switch (status) {
    case "active":
      return (
        <Badge variant="secondary" className="text-green-400 bg-green-500/10 border-green-500/20">
          Active
        </Badge>
      );
    case "expired":
      return (
        <Badge variant="secondary" className="text-yellow-400 bg-yellow-500/10 border-yellow-500/20">
          Expired
        </Badge>
      );
    case "revoked":
      return (
        <Badge variant="secondary" className="text-red-400 bg-red-500/10 border-red-500/20">
          Revoked
        </Badge>
      );
    default:
      return <Badge variant="secondary">{status}</Badge>;
  }
}

export default function TeamDetailPage() {
  const params = useParams();
  const orgId = params.id as string;
  const teamId = params.teamId as string;

  const [org, setOrg] = useState<Organization | null>(null);
  const [team, setTeam] = useState<Team | null>(null);
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [agentList, setAgentList] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);

  // Add member dialog
  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [addingMember, setAddingMember] = useState(false);
  const [memberForm, setMemberForm] = useState<AddTeamMemberRequest>({
    user_id: "",
    role: "member",
  });

  // Create agent dialog
  const [agentDialogOpen, setAgentDialogOpen] = useState(false);
  const [creatingAgent, setCreatingAgent] = useState(false);
  const [agentForm, setAgentForm] = useState<CreateAgentRequest>({
    name: "",
    description: "",
    scopes: [],
    ttl_hours: 0,
  });
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [tokenRevealed, setTokenRevealed] = useState(false);
  const { copied: tokenCopied, copy: copyToken } = useCopyToClipboard();

  // Removing
  const [removingMember, setRemovingMember] = useState<string | null>(null);
  const [deletingAgent, setDeletingAgent] = useState<string | null>(null);

  const loadData = async () => {
    try {
      const [orgData, teamData, membersData, agentsData] = await Promise.all([
        orgsApi.get(orgId),
        teamsApi.get(orgId, teamId),
        membersApi.list(orgId, teamId),
        agentsApi.list(orgId, teamId),
      ]);
      setOrg(orgData);
      setTeam(teamData);
      setMembers(membersData || []);
      setAgentList(agentsData || []);
    } catch (err) {
      toast.error("Failed to load team data");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId, teamId]);

  // ─── Member handlers ───────────────────────────────────────────────────────

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault();
    setAddingMember(true);
    try {
      await membersApi.add(orgId, teamId, memberForm);
      toast.success("Member added to team");
      setMemberDialogOpen(false);
      setMemberForm({ user_id: "", role: "member" });
      loadData();
    } catch (err) {
      toast.error("Failed to add member");
      console.error(err);
    } finally {
      setAddingMember(false);
    }
  };

  const handleRemoveMember = async (userId: string) => {
    setRemovingMember(userId);
    try {
      await membersApi.remove(orgId, teamId, userId);
      toast.success("Member removed from team");
      loadData();
    } catch (err) {
      toast.error("Failed to remove member");
      console.error(err);
    } finally {
      setRemovingMember(null);
    }
  };

  // ─── Agent handlers ────────────────────────────────────────────────────────

  const handleCreateAgent = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingAgent(true);
    try {
      const result = await agentsApi.create(orgId, teamId, agentForm);
      if (result.token) {
        setCreatedToken(result.token);
      } else {
        toast.success(`Agent "${agentForm.name}" created`);
        closeAgentDialog();
      }
      loadData();
    } catch (err) {
      toast.error("Failed to create agent");
      console.error(err);
    } finally {
      setCreatingAgent(false);
    }
  };

  const closeAgentDialog = () => {
    setAgentDialogOpen(false);
    setAgentForm({ name: "", description: "", scopes: [], ttl_hours: 0 });
    setCreatedToken(null);
    setTokenRevealed(false);
  };

  const handleRevokeAgent = async (agentId: string) => {
    try {
      await agentsApi.revoke(orgId, teamId, agentId);
      toast.success("Agent token revoked");
      loadData();
    } catch (err) {
      toast.error("Failed to revoke agent");
      console.error(err);
    }
  };

  const handleDeleteAgent = async (agentId: string) => {
    setDeletingAgent(agentId);
    try {
      await agentsApi.delete(orgId, teamId, agentId);
      toast.success("Agent deleted");
      loadData();
    } catch (err) {
      toast.error("Failed to delete agent");
      console.error(err);
    } finally {
      setDeletingAgent(null);
    }
  };

  const toggleScope = (scopeId: string) => {
    setAgentForm((f) => ({
      ...f,
      scopes: f.scopes.includes(scopeId)
        ? f.scopes.filter((s) => s !== scopeId)
        : [...f.scopes, scopeId],
    }));
  };

  if (loading) {
    return (
      <AppShell>
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <Link
            href={`/orgs/${orgId}`}
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ChevronLeft className="h-4 w-4" />
            {org?.name || "Organization"} / Teams
          </Link>

          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                <UsersRound className="h-6 w-6 text-primary" />
                {team?.name || "Team"}
              </h1>
              {team?.description && (
                <p className="text-muted-foreground mt-1">{team.description}</p>
              )}
              {team?.slug && (
                <Badge variant="secondary" className="mt-2 font-mono text-xs">
                  {team.slug}
                </Badge>
              )}
            </div>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="h-10 w-10 rounded-md bg-green-500/10 flex items-center justify-center">
                  <Users className="h-5 w-5 text-green-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{members.length}</p>
                  <p className="text-xs text-muted-foreground">Members</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="h-10 w-10 rounded-md bg-purple-500/10 flex items-center justify-center">
                  <Bot className="h-5 w-5 text-purple-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{agentList.length}</p>
                  <p className="text-xs text-muted-foreground">Agents</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Tabs: Members / Agents */}
        <Tabs defaultValue="members">
          <TabsList>
            <TabsTrigger value="members" className="gap-2">
              <Users className="h-4 w-4" />
              Members ({members.length})
            </TabsTrigger>
            <TabsTrigger value="agents" className="gap-2">
              <Bot className="h-4 w-4" />
              Agents ({agentList.length})
            </TabsTrigger>
          </TabsList>

          {/* ─── MEMBERS TAB ─── */}
          <TabsContent value="members" className="space-y-4 mt-4">
            <div className="flex justify-end">
              <Dialog open={memberDialogOpen} onOpenChange={setMemberDialogOpen}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    Add Member
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <form onSubmit={handleAddMember}>
                    <DialogHeader>
                      <DialogTitle>Add Team Member</DialogTitle>
                      <DialogDescription>
                        Add a user to this team with a specific role
                      </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                      <div className="space-y-2">
                        <Label htmlFor="member-user-id">User ID</Label>
                        <Input
                          id="member-user-id"
                          placeholder="UUID of the user to add"
                          value={memberForm.user_id}
                          onChange={(e) =>
                            setMemberForm((f) => ({
                              ...f,
                              user_id: e.target.value,
                            }))
                          }
                          required
                          autoFocus
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          Enter the user&apos;s UUID or email address
                        </p>
                      </div>
                      <div className="space-y-2">
                        <Label>Role</Label>
                        <Select
                          value={memberForm.role}
                          onValueChange={(v) =>
                            setMemberForm((f) => ({
                              ...f,
                              role: v as AddTeamMemberRequest["role"],
                            }))
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">Admin</SelectItem>
                            <SelectItem value="member">Member</SelectItem>
                            <SelectItem value="viewer">Viewer</SelectItem>
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground">
                          Admins can manage the team. Members have standard access. Viewers are read-only.
                        </p>
                      </div>
                    </div>
                    <DialogFooter>
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => setMemberDialogOpen(false)}
                      >
                        Cancel
                      </Button>
                      <Button type="submit" disabled={addingMember}>
                        {addingMember ? (
                          <>
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            Adding…
                          </>
                        ) : (
                          "Add Member"
                        )}
                      </Button>
                    </DialogFooter>
                  </form>
                </DialogContent>
              </Dialog>
            </div>

            {members.length === 0 ? (
              <Card className="border-dashed">
                <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                  <Users className="h-12 w-12 text-muted-foreground/50 mb-4" />
                  <h3 className="text-lg font-medium mb-1">No members yet</h3>
                  <p className="text-sm text-muted-foreground">
                    Add team members to collaborate on secrets
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="border rounded-lg">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>User</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead>Added</TableHead>
                      <TableHead className="w-[50px]" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {members.map((member) => {
                      const initials = member.user.name
                        .split(" ")
                        .map((n) => n[0])
                        .join("")
                        .toUpperCase()
                        .slice(0, 2);
                      return (
                        <TableRow key={member.id}>
                          <TableCell>
                            <div className="flex items-center gap-3">
                              <Avatar className="h-8 w-8">
                                <AvatarFallback className="text-xs bg-primary/20 text-primary">
                                  {initials}
                                </AvatarFallback>
                              </Avatar>
                              <div>
                                <p className="font-medium text-sm">
                                  {member.user.name}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  {member.user.email}
                                </p>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                member.role === "owner"
                                  ? "default"
                                  : member.role === "admin"
                                  ? "secondary"
                                  : "outline"
                              }
                              className="capitalize"
                            >
                              {member.role === "owner" && (
                                <Shield className="mr-1 h-3 w-3" />
                              )}
                              {member.role}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {formatDistanceToNow(new Date(member.added_at), {
                              addSuffix: true,
                            })}
                          </TableCell>
                          <TableCell>
                            {member.role !== "owner" && (
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 text-muted-foreground hover:text-destructive"
                                onClick={() =>
                                  handleRemoveMember(member.user_id)
                                }
                                disabled={removingMember === member.user_id}
                              >
                                {removingMember === member.user_id ? (
                                  <Loader2 className="h-4 w-4 animate-spin" />
                                ) : (
                                  <Trash2 className="h-4 w-4" />
                                )}
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            )}
          </TabsContent>

          {/* ─── AGENTS TAB ─── */}
          <TabsContent value="agents" className="space-y-4 mt-4">
            <div className="flex justify-end">
              <Dialog
                open={agentDialogOpen}
                onOpenChange={(open) => {
                  if (!open) closeAgentDialog();
                  else setAgentDialogOpen(true);
                }}
              >
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    New Agent
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-lg">
                  {createdToken ? (
                    <>
                      <DialogHeader>
                        <DialogTitle>Agent Token Created</DialogTitle>
                        <DialogDescription>
                          Copy this token now — you won&apos;t be able to see it
                          again.
                        </DialogDescription>
                      </DialogHeader>
                      <div className="py-4">
                        <div className="rounded-md bg-yellow-500/10 border border-yellow-500/20 px-4 py-3 mb-4 flex items-start gap-2">
                          <AlertTriangle className="h-4 w-4 text-yellow-500 mt-0.5 flex-shrink-0" />
                          <p className="text-sm text-yellow-200">
                            This token is shown only once. Store it securely.
                          </p>
                        </div>
                        <Label className="text-xs text-muted-foreground">
                          Agent Token
                        </Label>
                        <div className="flex items-center gap-2 mt-2">
                          <div className="flex-1 bg-muted/50 border rounded-md px-3 py-2 font-mono text-xs overflow-x-auto">
                            {tokenRevealed ? (
                              <span className="break-all select-all">
                                {createdToken}
                              </span>
                            ) : (
                              <span className="text-muted-foreground">
                                tvat.••••••••••••••••••••••••••••••••
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
                        <Button onClick={closeAgentDialog}>Done</Button>
                      </DialogFooter>
                    </>
                  ) : (
                    <form onSubmit={handleCreateAgent}>
                      <DialogHeader>
                        <DialogTitle>Create Agent</DialogTitle>
                        <DialogDescription>
                          Agents provide programmatic access with scoped
                          permissions
                        </DialogDescription>
                      </DialogHeader>
                      <div className="space-y-4 py-4">
                        <div className="space-y-2">
                          <Label htmlFor="agent-name">Name</Label>
                          <Input
                            id="agent-name"
                            placeholder="ci-deploy-bot"
                            value={agentForm.name}
                            onChange={(e) =>
                              setAgentForm((f) => ({
                                ...f,
                                name: e.target.value,
                              }))
                            }
                            required
                            autoFocus
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="agent-desc">
                            Description (optional)
                          </Label>
                          <Input
                            id="agent-desc"
                            placeholder="CI/CD deployment agent"
                            value={agentForm.description}
                            onChange={(e) =>
                              setAgentForm((f) => ({
                                ...f,
                                description: e.target.value,
                              }))
                            }
                          />
                        </div>
                        <div className="space-y-2">
                          <Label>Scopes</Label>
                          <div className="border rounded-md p-3 space-y-3 max-h-48 overflow-y-auto">
                            {AVAILABLE_SCOPES.map((scope) => (
                              <div
                                key={scope.id}
                                className="flex items-start gap-3"
                              >
                                <Checkbox
                                  id={`scope-${scope.id}`}
                                  checked={agentForm.scopes.includes(scope.id)}
                                  onCheckedChange={() => toggleScope(scope.id)}
                                />
                                <div className="grid gap-0.5 leading-none">
                                  <label
                                    htmlFor={`scope-${scope.id}`}
                                    className="text-sm font-medium cursor-pointer"
                                  >
                                    {scope.label}
                                  </label>
                                  <p className="text-xs text-muted-foreground">
                                    {scope.description}
                                  </p>
                                </div>
                              </div>
                            ))}
                          </div>
                          {agentForm.scopes.length === 0 && (
                            <p className="text-xs text-yellow-400">
                              Select at least one scope
                            </p>
                          )}
                        </div>
                        <div className="space-y-2">
                          <Label>Token TTL</Label>
                          <Select
                            value={String(agentForm.ttl_hours ?? 0)}
                            onValueChange={(v) =>
                              setAgentForm((f) => ({
                                ...f,
                                ttl_hours: parseInt(v),
                              }))
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {TTL_OPTIONS.map((opt) => (
                                <SelectItem key={opt.value} value={opt.value}>
                                  {opt.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <p className="text-xs text-muted-foreground">
                            How long the token remains valid
                          </p>
                        </div>
                      </div>
                      <DialogFooter>
                        <Button
                          type="button"
                          variant="outline"
                          onClick={closeAgentDialog}
                        >
                          Cancel
                        </Button>
                        <Button
                          type="submit"
                          disabled={
                            creatingAgent || agentForm.scopes.length === 0
                          }
                        >
                          {creatingAgent ? (
                            <>
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                              Creating…
                            </>
                          ) : (
                            "Create Agent"
                          )}
                        </Button>
                      </DialogFooter>
                    </form>
                  )}
                </DialogContent>
              </Dialog>
            </div>

            {agentList.length === 0 ? (
              <Card className="border-dashed">
                <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                  <Bot className="h-12 w-12 text-muted-foreground/50 mb-4" />
                  <h3 className="text-lg font-medium mb-1">No agents yet</h3>
                  <p className="text-sm text-muted-foreground">
                    Create an agent for CI/CD or programmatic access
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="border rounded-lg">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Scopes</TableHead>
                      <TableHead>Token Status</TableHead>
                      <TableHead>Expires</TableHead>
                      <TableHead>Last Used</TableHead>
                      <TableHead className="w-[80px]" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {agentList.map((agent) => (
                      <TableRow key={agent.id}>
                        <TableCell>
                          <div className="flex items-center gap-3">
                            <div className="h-8 w-8 rounded-md bg-purple-500/10 flex items-center justify-center">
                              <Bot className="h-4 w-4 text-purple-400" />
                            </div>
                            <div>
                              <p className="font-medium text-sm">
                                {agent.name}
                              </p>
                              {agent.description && (
                                <p className="text-xs text-muted-foreground line-clamp-1">
                                  {agent.description}
                                </p>
                              )}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {agent.scopes.map((scope) => (
                              <Badge
                                key={scope}
                                variant="outline"
                                className="font-mono text-xs"
                              >
                                {scope}
                              </Badge>
                            ))}
                          </div>
                        </TableCell>
                        <TableCell>
                          <TokenStatusBadge status={agent.token_status} />
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {agent.expires_at ? (
                            <span className="flex items-center gap-1">
                              <Clock className="h-3.5 w-3.5" />
                              {format(new Date(agent.expires_at), "PP")}
                            </span>
                          ) : (
                            "Never"
                          )}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {agent.last_used_at
                            ? formatDistanceToNow(
                                new Date(agent.last_used_at),
                                { addSuffix: true }
                              )
                            : "Never"}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            {agent.token_status === "active" && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 text-xs text-yellow-400 hover:text-yellow-300"
                                onClick={() => handleRevokeAgent(agent.id)}
                              >
                                Revoke
                              </Button>
                            )}
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8 text-muted-foreground hover:text-destructive"
                              onClick={() => handleDeleteAgent(agent.id)}
                              disabled={deletingAgent === agent.id}
                            >
                              {deletingAgent === agent.id ? (
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
        </Tabs>
      </div>
    </AppShell>
  );
}
