"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  orgs as orgsApi,
  teams as teamsApi,
  type Organization,
  type Team,
  type CreateTeamRequest,
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
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Building2,
  ChevronLeft,
  Plus,
  Loader2,
  Users,
  UsersRound,
  Bot,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

export default function OrgDetailPage() {
  const params = useParams();
  const router = useRouter();
  const orgId = params.id as string;

  const [org, setOrg] = useState<Organization | null>(null);
  const [teamList, setTeamList] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<CreateTeamRequest>({
    name: "",
    slug: "",
    description: "",
  });

  const loadOrg = async () => {
    try {
      const data = await orgsApi.get(orgId);
      setOrg(data);
    } catch (err) {
      toast.error("Failed to load organization");
      console.error(err);
    }
  };

  const loadTeams = async () => {
    try {
      const data = await teamsApi.list(orgId);
      setTeamList(data || []);
    } catch (err) {
      toast.error("Failed to load teams");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOrg();
    loadTeams();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId]);

  const generateSlug = (name: string) => {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
  };

  const handleCreateTeam = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      await teamsApi.create(orgId, form);
      toast.success(`Team "${form.name}" created`);
      setDialogOpen(false);
      setForm({ name: "", slug: "", description: "" });
      loadTeams();
    } catch (err) {
      toast.error("Failed to create team");
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <Link
            href="/orgs"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ChevronLeft className="h-4 w-4" />
            Back to organizations
          </Link>

          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                <Building2 className="h-6 w-6 text-primary" />
                {org?.name || "Organization"}
              </h1>
              {org?.description && (
                <p className="text-muted-foreground mt-1">
                  {org.description}
                </p>
              )}
              {org?.slug && (
                <Badge variant="secondary" className="mt-2 font-mono text-xs">
                  {org.slug}
                </Badge>
              )}
            </div>

            <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
              <DialogTrigger asChild>
                <Button>
                  <Plus className="mr-2 h-4 w-4" />
                  New Team
                </Button>
              </DialogTrigger>
              <DialogContent>
                <form onSubmit={handleCreateTeam}>
                  <DialogHeader>
                    <DialogTitle>Create Team</DialogTitle>
                    <DialogDescription>
                      Teams organize members and agents within this organization
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label htmlFor="team-name">Name</Label>
                      <Input
                        id="team-name"
                        placeholder="Backend Engineering"
                        value={form.name}
                        onChange={(e) => {
                          const name = e.target.value;
                          setForm((f) => ({
                            ...f,
                            name,
                            slug:
                              f.slug === generateSlug(f.name) || !f.slug
                                ? generateSlug(name)
                                : f.slug,
                          }));
                        }}
                        required
                        autoFocus
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="team-slug">Slug</Label>
                      <Input
                        id="team-slug"
                        placeholder="backend-engineering"
                        value={form.slug}
                        onChange={(e) =>
                          setForm((f) => ({ ...f, slug: e.target.value }))
                        }
                        className="font-mono"
                      />
                      <p className="text-xs text-muted-foreground">
                        URL-friendly identifier
                      </p>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="team-desc">Description</Label>
                      <Textarea
                        id="team-desc"
                        placeholder="What this team does…"
                        value={form.description}
                        onChange={(e) =>
                          setForm((f) => ({
                            ...f,
                            description: e.target.value,
                          }))
                        }
                        rows={3}
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
                        "Create Team"
                      )}
                    </Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>
        </div>

        {/* Stats row */}
        {org && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <div className="h-10 w-10 rounded-md bg-blue-500/10 flex items-center justify-center">
                    <UsersRound className="h-5 w-5 text-blue-400" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold">{teamList.length}</p>
                    <p className="text-xs text-muted-foreground">Teams</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <div className="h-10 w-10 rounded-md bg-green-500/10 flex items-center justify-center">
                    <Users className="h-5 w-5 text-green-400" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold">{org.member_count ?? 0}</p>
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
                    <p className="text-2xl font-bold">
                      {teamList.reduce((sum, t) => sum + (t.agent_count ?? 0), 0)}
                    </p>
                    <p className="text-xs text-muted-foreground">Agents</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Teams table */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : teamList.length === 0 ? (
          <Card className="border-dashed">
            <CardContent className="flex flex-col items-center justify-center py-16 text-center">
              <UsersRound className="h-12 w-12 text-muted-foreground/50 mb-4" />
              <h3 className="text-lg font-medium mb-1">No teams yet</h3>
              <p className="text-sm text-muted-foreground mb-4">
                Create your first team to start organizing members
              </p>
              <Button onClick={() => setDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                New Team
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="border rounded-lg">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Team</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead>Members</TableHead>
                  <TableHead>Agents</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {teamList.map((team) => (
                  <TableRow
                    key={team.id}
                    className="cursor-pointer"
                    onClick={() =>
                      router.push(`/orgs/${orgId}/teams/${team.id}`)
                    }
                  >
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <div className="h-8 w-8 rounded-md bg-primary/10 flex items-center justify-center">
                          <UsersRound className="h-4 w-4 text-primary" />
                        </div>
                        <div>
                          <p className="font-medium">{team.name}</p>
                          {team.description && (
                            <p className="text-xs text-muted-foreground line-clamp-1">
                              {team.description}
                            </p>
                          )}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="font-mono text-xs">
                        {team.slug}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1 text-sm">
                        <Users className="h-3.5 w-3.5 text-muted-foreground" />
                        {team.member_count ?? 0}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1 text-sm">
                        <Bot className="h-3.5 w-3.5 text-muted-foreground" />
                        {team.agent_count ?? 0}
                      </span>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDistanceToNow(new Date(team.created_at), {
                        addSuffix: true,
                      })}
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
