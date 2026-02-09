"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  orgs as orgsApi,
  type Organization,
  type CreateOrganizationRequest,
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
  Building2,
  Plus,
  Loader2,
  Users,
  UsersRound,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

export default function OrgsPage() {
  const router = useRouter();
  const [orgList, setOrgList] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<CreateOrganizationRequest>({
    name: "",
    slug: "",
    description: "",
  });

  const loadOrgs = async () => {
    try {
      const data = await orgsApi.list();
      setOrgList(data || []);
    } catch (err) {
      toast.error("Failed to load organizations");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOrgs();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      const created = await orgsApi.create(form);
      toast.success(`Organization "${form.name}" created`);
      setDialogOpen(false);
      setForm({ name: "", slug: "", description: "" });
      loadOrgs();
    } catch (err) {
      toast.error("Failed to create organization");
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const generateSlug = (name: string) => {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
  };

  return (
    <AppShell>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
              <Building2 className="h-6 w-6 text-primary" />
              Organizations
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage your organizations and teams
            </p>
          </div>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                New Organization
              </Button>
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={handleCreate}>
                <DialogHeader>
                  <DialogTitle>Create Organization</DialogTitle>
                  <DialogDescription>
                    Organizations group teams and manage access centrally
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label htmlFor="org-name">Name</Label>
                    <Input
                      id="org-name"
                      placeholder="Acme Corp"
                      value={form.name}
                      onChange={(e) => {
                        const name = e.target.value;
                        setForm((f) => ({
                          ...f,
                          name,
                          slug: f.slug === generateSlug(f.name) || !f.slug
                            ? generateSlug(name)
                            : f.slug,
                        }));
                      }}
                      required
                      autoFocus
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="org-slug">Slug</Label>
                    <Input
                      id="org-slug"
                      placeholder="acme-corp"
                      value={form.slug}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, slug: e.target.value }))
                      }
                      className="font-mono"
                    />
                    <p className="text-xs text-muted-foreground">
                      URL-friendly identifier. Auto-generated from name.
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="org-desc">Description</Label>
                    <Textarea
                      id="org-desc"
                      placeholder="What this organization is for…"
                      value={form.description}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, description: e.target.value }))
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
                      "Create Organization"
                    )}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {/* Org grid */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : orgList.length === 0 ? (
          <Card className="border-dashed">
            <CardContent className="flex flex-col items-center justify-center py-16 text-center">
              <Building2 className="h-12 w-12 text-muted-foreground/50 mb-4" />
              <h3 className="text-lg font-medium mb-1">No organizations yet</h3>
              <p className="text-sm text-muted-foreground mb-4">
                Create your first organization to start managing teams
              </p>
              <Button onClick={() => setDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                New Organization
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {orgList.map((org) => (
              <Card
                key={org.id}
                className="cursor-pointer hover:border-primary/50 transition-colors group"
                onClick={() => router.push(`/orgs/${org.id}`)}
              >
                <CardHeader className="pb-3">
                  <div className="flex items-start gap-3">
                    <div className="h-9 w-9 rounded-md bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition-colors">
                      <Building2 className="h-5 w-5 text-primary" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <CardTitle className="text-base truncate">
                        {org.name}
                      </CardTitle>
                      <CardDescription className="text-xs mt-1 font-mono">
                        {org.slug}
                      </CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-0">
                  {org.description && (
                    <p className="text-sm text-muted-foreground line-clamp-2 mb-3">
                      {org.description}
                    </p>
                  )}
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <Users className="h-3.5 w-3.5" />
                      {org.member_count ?? 0} members
                    </span>
                    <span className="flex items-center gap-1">
                      <UsersRound className="h-3.5 w-3.5" />
                      {org.team_count ?? 0} teams
                    </span>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </AppShell>
  );
}
