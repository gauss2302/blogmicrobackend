"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { getPost } from "@/lib/posts/client";
import { cn } from "@/lib/utils";

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

export default function PostDetailPage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";

  const { data: post, isLoading, error } = useQuery({
    queryKey: ["post", id],
    queryFn: () => getPost(id),
    enabled: Boolean(id),
  });

  if (isLoading || !id) {
    return (
      <div className="flex flex-col">
        <header className="h-14 shrink-0 border-b border-zinc-800/80 bg-zinc-950/60 px-6" />
        <div className="mx-auto flex min-h-[40vh] w-full max-w-2xl flex-col items-center justify-center gap-3 px-6">
          <Loader2 className="h-8 w-8 animate-spin text-cyan-300" />
          <p className="text-sm text-zinc-400">Loading post…</p>
        </div>
      </div>
    );
  }

  if (error || !post) {
    return (
      <div className="flex flex-col">
        <header className="flex h-14 shrink-0 items-center border-b border-zinc-800/80 bg-zinc-950/60 px-6">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/app">
              <ArrowLeft className="h-4 w-4" />
              Back
            </Link>
          </Button>
        </header>
        <div className="mx-auto w-full max-w-2xl px-6 py-8">
          <p className="rounded-md border border-rose-600/40 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
            {error instanceof Error ? error.message : "Post not found."}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col">
      <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center border-b border-zinc-800/80 bg-zinc-950/60 px-6 backdrop-blur-sm">
        <Button variant="ghost" size="sm" className="-ml-2" asChild>
          <Link href="/app">
            <ArrowLeft className="h-4 w-4" />
            Back
          </Link>
        </Button>
      </header>
      <div className="mx-auto w-full max-w-2xl px-6 py-8">
      <Card className="border-zinc-700/60">
        <CardHeader className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle className="text-2xl">{post.title}</CardTitle>
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
          <p className={cn("text-xs text-zinc-500")}>
            /{post.slug || post.id} · {post.user_id} · {formatDate(post.created_at)}
          </p>
        </CardHeader>
        <CardContent>
          <div className="prose prose-invert max-w-none text-sm text-zinc-300 whitespace-pre-wrap">
            {post.content}
          </div>
        </CardContent>
      </Card>
      </div>
    </div>
  );
}
