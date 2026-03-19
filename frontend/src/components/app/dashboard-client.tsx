"use client";

import Link from "next/link";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Plus, Search, UserPlus, UserMinus } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Modal } from "@/components/ui/modal";
import { useAuthStore } from "@/lib/stores/auth-store";
import { createPost, listPosts, listUserPosts } from "@/lib/posts/client";
import type { PostSummary } from "@/lib/posts/types";
import { search, followUser, unfollowUser, type SearchUserHit, type SearchPostHit } from "@/lib/search/client";
import { cn } from "@/lib/utils";

const POSTS_ALL_QUERY_KEY = ["posts-all"] as const;
const POSTS_MINE_QUERY_KEY = (userId: string) => ["posts-mine", userId] as const;
const SEARCH_QUERY_KEY = (q: string, uc?: string, pc?: string) => ["search", q, uc ?? "", pc ?? ""] as const;

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return iso;
  }
}

function PostCard({ post }: { post: PostSummary }) {
  return (
    <Link
      href={`/app/post/${post.id}`}
      className="block transition-opacity hover:opacity-90"
    >
      <Card className="border-zinc-700/60">
        <CardHeader className="pb-2">
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle className="text-base">{post.title}</CardTitle>
            {post.published ? (
              <Badge className="bg-emerald-500/20 text-emerald-300 border-emerald-500/40">
                Published
              </Badge>
            ) : (
              <Badge className="border-amber-500/40 bg-amber-500/10 text-amber-300">
                Draft
              </Badge>
            )}
          </div>
          <CardDescription className="text-xs text-zinc-500">
            /{post.slug || post.id} · {post.user_id.slice(0, 8)}… · {formatDate(post.created_at)}
          </CardDescription>
        </CardHeader>
      </Card>
    </Link>
  );
}

function PostsGrid({
  posts,
  isLoading,
  error,
  emptyMessage,
}: {
  posts: PostSummary[];
  isLoading: boolean;
  error: Error | null;
  emptyMessage: string;
}) {
  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2">
        {[1, 2, 3].map((i) => (
          <Card key={i} className="border-zinc-700/60 animate-pulse">
            <CardHeader>
              <div className="h-4 w-3/4 rounded bg-zinc-700" />
              <div className="mt-2 h-3 w-1/2 rounded bg-zinc-800" />
            </CardHeader>
          </Card>
        ))}
      </div>
    );
  }
  if (error) {
    return (
      <p className="rounded-md border border-rose-600/40 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
        {error.message}
      </p>
    );
  }
  if (!posts.length) {
    return (
      <p className="rounded-md border border-zinc-700 bg-zinc-900/50 px-4 py-6 text-center text-sm text-zinc-400">
        {emptyMessage}
      </p>
    );
  }
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {posts.map((post) => (
        <PostCard key={post.id} post={post} />
      ))}
    </div>
  );
}

function DiscoverSection() {
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [usersCursor, setUsersCursor] = useState("");
  const [postsCursor, setPostsCursor] = useState("");
  const [subscribedIds, setSubscribedIds] = useState<Set<string>>(new Set());

  const searchQuery = useQuery({
    queryKey: SEARCH_QUERY_KEY(submittedQuery, usersCursor, postsCursor),
    queryFn: () =>
      search({
        q: submittedQuery,
        usersLimit: 10,
        postsLimit: 10,
        usersCursor: usersCursor || undefined,
        postsCursor: postsCursor || undefined,
      }),
    enabled: submittedQuery.length > 0,
  });

  const followMutation = useMutation({
    mutationFn: followUser,
    onSuccess: (_, userId) => {
      setSubscribedIds((prev) => new Set(prev).add(userId));
      queryClient.invalidateQueries({ queryKey: ["search"] });
    },
  });

  const unfollowMutation = useMutation({
    mutationFn: unfollowUser,
    onSuccess: (_, userId) => {
      setSubscribedIds((prev) => {
        const next = new Set(prev);
        next.delete(userId);
        return next;
      });
      queryClient.invalidateQueries({ queryKey: ["search"] });
    },
  });

  const data = searchQuery.data;
  const users = data?.users ?? [];
  const posts = data?.posts ?? [];
  const usersNext = data?.users_next_cursor ?? "";
  const postsNext = data?.posts_next_cursor ?? "";

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <Input
            placeholder="Search users and posts…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && setSubmittedQuery(query.trim())}
            className="pl-9"
          />
        </div>
        <Button
          type="button"
          onClick={() => setSubmittedQuery(query.trim())}
          disabled={!query.trim()}
        >
          Search
        </Button>
      </div>

      {searchQuery.isLoading && (
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Card key={i} className="border-zinc-700/60 animate-pulse">
              <CardHeader>
                <div className="h-4 w-2/3 rounded bg-zinc-700" />
                <div className="mt-2 h-3 w-1/2 rounded bg-zinc-800" />
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      {searchQuery.error && (
        <p className="rounded-md border border-rose-600/40 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
          {(searchQuery.error as Error).message}
        </p>
      )}

      {data && submittedQuery && !searchQuery.isLoading && (
        <>
          {users.length > 0 && (
            <div>
              <h2 className="mb-3 text-sm font-medium text-zinc-400">Users</h2>
              <div className="grid gap-3 sm:grid-cols-2">
                {users.map((u: SearchUserHit) => (
                  <Card key={u.id} className="border-zinc-700/60">
                    <CardHeader className="flex flex-row items-start justify-between gap-2 pb-2">
                      <div className="min-w-0 flex-1">
                        <CardTitle className="truncate text-base">{u.name || u.id}</CardTitle>
                        <CardDescription className="truncate text-xs text-zinc-500">
                          {u.bio || "No bio"}
                        </CardDescription>
                      </div>
                      {user?.id !== u.id && (
                        <Button
                          variant={subscribedIds.has(u.id) ? "outline" : "default"}
                          size="sm"
                          className="shrink-0"
                          onClick={() =>
                            subscribedIds.has(u.id)
                              ? unfollowMutation.mutate(u.id)
                              : followMutation.mutate(u.id)
                          }
                          disabled={followMutation.isPending || unfollowMutation.isPending}
                        >
                          {subscribedIds.has(u.id) ? (
                            <>
                              <UserMinus className="mr-1 h-3.5 w-3.5" />
                              Unsubscribe
                            </>
                          ) : (
                            <>
                              <UserPlus className="mr-1 h-3.5 w-3.5" />
                              Subscribe
                            </>
                          )}
                        </Button>
                      )}
                    </CardHeader>
                  </Card>
                ))}
              </div>
              {usersNext && (
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2"
                  onClick={() => setUsersCursor(usersNext)}
                  disabled={searchQuery.isFetching}
                >
                  Load more users
                </Button>
              )}
            </div>
          )}

          {posts.length > 0 && (
            <div>
              <h2 className="mb-3 text-sm font-medium text-zinc-400">Posts</h2>
              <div className="grid gap-4 sm:grid-cols-2">
                {posts.map((p: SearchPostHit) => (
                  <Link key={p.id} href={`/app/post/${p.id}`} className="block transition-opacity hover:opacity-90">
                    <Card className="border-zinc-700/60">
                      <CardHeader className="pb-2">
                        <CardTitle className="text-base">{p.title}</CardTitle>
                        <CardDescription className="line-clamp-2 text-xs text-zinc-500">
                          {p.content_preview || p.slug}
                        </CardDescription>
                      </CardHeader>
                    </Card>
                  </Link>
                ))}
              </div>
              {postsNext && (
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2"
                  onClick={() => setPostsCursor(postsNext)}
                  disabled={searchQuery.isFetching}
                >
                  Load more posts
                </Button>
              )}
            </div>
          )}

          {users.length === 0 && posts.length === 0 && (
            <p className="rounded-md border border-zinc-700 bg-zinc-900/50 px-4 py-6 text-center text-sm text-zinc-400">
              No users or posts found for &quot;{submittedQuery}&quot;.
            </p>
          )}
        </>
      )}

      {!submittedQuery && (
        <p className="rounded-md border border-zinc-700 bg-zinc-900/50 px-4 py-6 text-center text-sm text-zinc-400">
          Enter a search term to discover users and posts.
        </p>
      )}
    </div>
  );
}

export function DashboardClient() {
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const [tab, setTab] = useState<"all" | "mine" | "discover">("all");
  const [modalOpen, setModalOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [published, setPublished] = useState(true);
  const [formError, setFormError] = useState<string | null>(null);

  const allPostsQuery = useQuery({
    queryKey: POSTS_ALL_QUERY_KEY,
    queryFn: () => listPosts({ limit: 20, offset: 0, publishedOnly: true }),
  });

  const myPostsQuery = useQuery({
    queryKey: user?.id ? POSTS_MINE_QUERY_KEY(user.id) : ["posts-mine-disabled"],
    queryFn: () => listUserPosts(user!.id, { limit: 20, offset: 0 }),
    enabled: Boolean(user?.id),
  });

  const createMutation = useMutation({
    mutationFn: createPost,
    onSuccess: () => {
      setTitle("");
      setContent("");
      setPublished(true);
      setFormError(null);
      setModalOpen(false);
      queryClient.invalidateQueries({ queryKey: POSTS_ALL_QUERY_KEY });
      if (user?.id) {
        queryClient.invalidateQueries({ queryKey: POSTS_MINE_QUERY_KEY(user.id) });
      }
    },
    onError: (err: Error) => {
      setFormError(err.message);
    },
  });

  const posts =
    tab === "all"
      ? allPostsQuery.data?.posts ?? []
      : tab === "mine"
        ? myPostsQuery.data?.posts ?? []
        : [];
  const isLoading = tab === "all" ? allPostsQuery.isLoading : tab === "mine" ? myPostsQuery.isLoading : false;
  const error = tab === "all" ? allPostsQuery.error : tab === "mine" ? myPostsQuery.error : null;

  return (
    <div className="flex flex-col">
      <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center justify-between border-b border-zinc-800/80 bg-zinc-950/60 px-6 backdrop-blur-sm">
        <h1 className="text-sm font-semibold text-zinc-300">Feed</h1>
        <Button
          variant="default"
          size="sm"
          onClick={() => setModalOpen(true)}
          aria-label="Create post"
          className="h-9 w-9 rounded-full p-0"
        >
          <Plus className="h-5 w-5" />
        </Button>
      </header>

      <Modal
        open={modalOpen}
        onClose={() => {
          setModalOpen(false);
          setFormError(null);
        }}
        title="Create post"
        description="Add a new post with title and content. Choose whether to publish now or save as draft."
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setFormError(null);
            createMutation.mutate({ title, content, published });
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="post-title">Title</Label>
            <Input
              id="post-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Post title"
              maxLength={200}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="post-content">Content</Label>
            <textarea
              id="post-content"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="Write your post content…"
              required
              rows={4}
              className={cn(
                "w-full rounded-md border border-zinc-700 bg-zinc-950/70 px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/80",
              )}
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="post-published"
              checked={published}
              onChange={(e) => setPublished(e.target.checked)}
              className="h-4 w-4 rounded border-zinc-600 bg-zinc-900 text-cyan-500 focus:ring-cyan-400/80"
            />
            <Label htmlFor="post-published" className="cursor-pointer text-zinc-300">
              Publish immediately
            </Label>
          </div>
          {formError && (
            <p className="text-sm text-rose-300">{formError}</p>
          )}
          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={createMutation.isPending}>
              {createMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Create post"
              )}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setModalOpen(false);
                setFormError(null);
              }}
            >
              Cancel
            </Button>
          </div>
        </form>
      </Modal>

      <div className="mx-auto w-full max-w-4xl flex-1 px-6 py-8">
        <div className="space-y-6">
          <div className="flex gap-1 rounded-lg bg-zinc-900/60 p-1">
            <button
              type="button"
              onClick={() => setTab("all")}
              className={cn(
                "rounded-md px-4 py-2 text-sm font-medium transition-colors",
                tab === "all"
                  ? "bg-zinc-700/80 text-zinc-100"
                  : "text-zinc-400 hover:text-zinc-200",
              )}
            >
              All posts
            </button>
            <button
              type="button"
              onClick={() => setTab("mine")}
              className={cn(
                "rounded-md px-4 py-2 text-sm font-medium transition-colors",
                tab === "mine"
                  ? "bg-zinc-700/80 text-zinc-100"
                  : "text-zinc-400 hover:text-zinc-200",
              )}
            >
              My posts
            </button>
            <button
              type="button"
              onClick={() => setTab("discover")}
              className={cn(
                "rounded-md px-4 py-2 text-sm font-medium transition-colors",
                tab === "discover"
                  ? "bg-zinc-700/80 text-zinc-100"
                  : "text-zinc-400 hover:text-zinc-200",
              )}
            >
              Discover
            </button>
          </div>
          {tab === "discover" ? (
            <DiscoverSection />
          ) : (
            <PostsGrid
            posts={posts}
            isLoading={isLoading}
            error={error instanceof Error ? error : null}
            emptyMessage={tab === "all" ? "No posts yet." : "You haven’t created any posts yet."}
            />
          )}
        </div>
      </div>
    </div>
  );
}
