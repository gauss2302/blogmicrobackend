"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useThemeStore } from "@/lib/theme/theme-store";
import { getResolvedTheme } from "@/lib/theme/apply-theme";
import { useAuthStore } from "@/lib/stores/auth-store";
import {
  deleteAccount,
  getUser,
  listFollowers,
  listFollowing,
  updateUser,
} from "@/lib/users/client";
import type { ThemePreference } from "@/lib/theme/constants";
import { Moon, Sun, Monitor } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

const THEME_OPTIONS: { value: ThemePreference; label: string; icon: React.ReactNode }[] = [
  { value: "dark", label: "Dark", icon: <Moon className="h-4 w-4" /> },
  { value: "light", label: "Light", icon: <Sun className="h-4 w-4" /> },
  { value: "system", label: "System", icon: <Monitor className="h-4 w-4" /> },
];

export default function ProfilePage() {
  const user = useAuthStore((state) => state.user);
  const preference = useThemeStore((state) => state.preference);
  const setPreference = useThemeStore((state) => state.setPreference);
  // Safe to read matchMedia here: SessionBootstrap renders this page on the
  // client only, so there is no server markup to mismatch.
  const resolved = getResolvedTheme(preference);
  const queryClient = useQueryClient();
  const userId = user?.id;

  // The JWT-derived store only carries id/email/name. The full record (bio,
  // location, website) lives in user-service, so read it properly.
  const profileQuery = useQuery({
    queryKey: ["user", userId],
    queryFn: () => getUser(userId as string),
    enabled: Boolean(userId),
  });

  // The follow endpoints are cursor-paginated and return no total, so an exact
  // count would mean walking every page. Fetch one full page instead and show
  // "100+" when more remain — honest, and one request.
  const FOLLOW_PAGE = 100;

  const followersQuery = useQuery({
    queryKey: ["followers", userId],
    queryFn: () => listFollowers(userId as string, { limit: FOLLOW_PAGE }),
    enabled: Boolean(userId),
  });

  const followingQuery = useQuery({
    queryKey: ["following", userId],
    queryFn: () => listFollowing(userId as string, { limit: FOLLOW_PAGE }),
    enabled: Boolean(userId),
  });

  const [form, setForm] = useState({
    name: "",
    bio: "",
    location: "",
    website: "",
  });
  const [isEditing, setIsEditing] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Seed the form once the record arrives, and re-seed after a save so the
  // inputs reflect whatever the backend actually stored (it may normalise).
  // Adjusted during render, not in an effect, per React's guidance.
  const profile = profileQuery.data;
  const [seededFrom, setSeededFrom] = useState<typeof profile>(undefined);
  if (profile && profile !== seededFrom) {
    setSeededFrom(profile);
    setForm({
      name: profile.name ?? "",
      bio: profile.bio ?? "",
      location: profile.location ?? "",
      website: profile.website ?? "",
    });
  }

  const router = useRouter();
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const deleteMutation = useMutation({
    mutationFn: () => deleteAccount(userId as string),
    onSuccess: () => {
      // The BFF already cleared the session cookies; drop client state too so
      // nothing renders stale user data on the way out.
      useAuthStore.getState().clearSession();
      queryClient.clear();
      router.replace("/auth/login?deleted=1");
    },
    onError: (error: Error) => setDeleteError(error.message),
  });

  const saveMutation = useMutation({
    mutationFn: () => updateUser(userId as string, form),
    onSuccess: (updated) => {
      setFormError(null);
      setIsEditing(false);
      queryClient.setQueryData(["user", userId], updated);
      // The sidebar and greeting read the name from the auth store.
      const current = useAuthStore.getState().user;
      if (current) {
        useAuthStore.setState({ user: { ...current, name: updated.name } });
      }
    },
    onError: (error: Error) => setFormError(error.message),
  });

  return (
    <div className="flex flex-col">
      <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center border-b border-border bg-card/80 px-6 backdrop-blur-sm">
        <h1 className="text-sm font-semibold text-foreground">Profile</h1>
      </header>
      <div className="mx-auto w-full max-w-2xl px-6 py-8">
        <div className="space-y-6">
          <Card className="border-border bg-card">
            <CardHeader className="flex flex-row items-start justify-between gap-4">
              <div>
                <CardTitle className="text-foreground">Account</CardTitle>
                <CardDescription>
                  Your public profile. Email is managed by your sign-in method.
                </CardDescription>
              </div>
              {!isEditing && profile && (
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setIsEditing(true)}
                >
                  Edit
                </Button>
              )}
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <div>
                <span className="text-muted">Email</span>
                <p className="font-medium text-foreground">
                  {profile?.email ?? user?.email ?? "—"}
                </p>
              </div>

              {profileQuery.isLoading && (
                <p className="text-muted">Loading profile…</p>
              )}
              {profileQuery.isError && (
                <p className="text-destructive">
                  {(profileQuery.error as Error).message}
                </p>
              )}

              {profile && !isEditing && (
                <>
                  <ProfileField label="Name" value={profile.name} />
                  <ProfileField label="Bio" value={profile.bio} />
                  <ProfileField label="Location" value={profile.location} />
                  <ProfileField label="Website" value={profile.website} />
                </>
              )}

              {profile && isEditing && (
                <form
                  className="space-y-4"
                  onSubmit={(event) => {
                    event.preventDefault();
                    saveMutation.mutate();
                  }}
                >
                  <div className="space-y-1.5">
                    <Label htmlFor="profile-name">Name</Label>
                    <Input
                      id="profile-name"
                      value={form.name}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, name: e.target.value }))
                      }
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="profile-bio">Bio</Label>
                    <Input
                      id="profile-bio"
                      value={form.bio}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, bio: e.target.value }))
                      }
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="profile-location">Location</Label>
                    <Input
                      id="profile-location"
                      value={form.location}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, location: e.target.value }))
                      }
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="profile-website">Website</Label>
                    <Input
                      id="profile-website"
                      placeholder="https://example.com"
                      value={form.website}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, website: e.target.value }))
                      }
                    />
                  </div>

                  {formError && (
                    <p className="text-destructive">{formError}</p>
                  )}

                  <div className="flex gap-2">
                    <Button type="submit" disabled={saveMutation.isPending}>
                      {saveMutation.isPending ? "Saving…" : "Save changes"}
                    </Button>
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={saveMutation.isPending}
                      onClick={() => {
                        setIsEditing(false);
                        setFormError(null);
                        setForm({
                          name: profile.name ?? "",
                          bio: profile.bio ?? "",
                          location: profile.location ?? "",
                          website: profile.website ?? "",
                        });
                      }}
                    >
                      Cancel
                    </Button>
                  </div>
                </form>
              )}

              {user?.id && (
                <div>
                  <span className="text-muted">User ID</span>
                  <p className="font-mono text-xs text-foreground">{user.id}</p>
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-border bg-card">
            <CardHeader>
              <CardTitle className="text-foreground">Network</CardTitle>
              <CardDescription>Your follow graph.</CardDescription>
            </CardHeader>
            <CardContent className="flex gap-8 text-sm">
              <FollowCount
                label="Followers"
                isLoading={followersQuery.isLoading}
                count={followersQuery.data?.users.length}
                hasMore={Boolean(followersQuery.data?.next_cursor)}
              />
              <FollowCount
                label="Following"
                isLoading={followingQuery.isLoading}
                count={followingQuery.data?.users.length}
                hasMore={Boolean(followingQuery.data?.next_cursor)}
              />
            </CardContent>
          </Card>

          <Card className="border-destructive/40 bg-card">
            <CardHeader>
              <CardTitle className="text-destructive">Danger zone</CardTitle>
              <CardDescription>
                Deleting your account removes your profile, takes all your
                posts down and signs you out on every device. This cannot be
                undone.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div className="space-y-1.5">
                <Label htmlFor="delete-confirm">
                  Type your email to confirm
                </Label>
                <Input
                  id="delete-confirm"
                  autoComplete="off"
                  placeholder={profile?.email ?? user?.email ?? ""}
                  value={deleteConfirm}
                  onChange={(e) => setDeleteConfirm(e.target.value)}
                />
              </div>

              {deleteError && <p className="text-destructive">{deleteError}</p>}

              <Button
                type="button"
                variant="destructive"
                // Requiring the exact email makes an accidental click on an
                // irreversible action effectively impossible.
                disabled={
                  deleteMutation.isPending ||
                  deleteConfirm.trim().toLowerCase() !==
                    (profile?.email ?? user?.email ?? "").toLowerCase() ||
                  !(profile?.email ?? user?.email)
                }
                onClick={() => {
                  setDeleteError(null);
                  deleteMutation.mutate();
                }}
              >
                {deleteMutation.isPending ? "Deleting…" : "Delete account"}
              </Button>
            </CardContent>
          </Card>

          <Card className="border-border bg-card">
            <CardHeader>
              <CardTitle className="text-foreground">Appearance</CardTitle>
              <CardDescription>
                Choose how the app looks. Current: {resolved === "dark" ? "Dark" : "Light"}.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {THEME_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setPreference(opt.value)}
                    className={cn(
                      "flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors",
                      preference === opt.value
                        ? "border-ring bg-ring/15 text-foreground"
                        : "border-border bg-transparent text-muted hover:bg-background hover:text-foreground",
                    )}
                  >
                    {opt.icon}
                    {opt.label}
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function ProfileField({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <span className="text-muted">{label}</span>
      <p className="font-medium text-foreground">
        {value && value.trim() ? value : "—"}
      </p>
    </div>
  );
}

function FollowCount({
  label,
  isLoading,
  count,
  hasMore,
}: {
  label: string;
  isLoading: boolean;
  count?: number;
  hasMore: boolean;
}) {
  const display = isLoading
    ? "…"
    : `${count ?? 0}${hasMore ? "+" : ""}`;

  return (
    <div>
      <span className="text-muted">{label}</span>
      <p className="font-medium text-foreground">{display}</p>
    </div>
  );
}
