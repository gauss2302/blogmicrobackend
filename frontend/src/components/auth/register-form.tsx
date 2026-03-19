"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Loader2, UserPlus } from "lucide-react";
import { motion } from "framer-motion";

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
import {
  getGoogleAuthURL,
  registerWithPassword,
} from "@/lib/auth/client-api";
import { OAUTH_CLIENT_STATE_STORAGE_KEY } from "@/lib/auth/client-constants";

function buildClientState() {
  if (typeof window === "undefined") {
    return "";
  }

  if (window.crypto && "randomUUID" in window.crypto) {
    return window.crypto.randomUUID();
  }

  return Math.random().toString(36).slice(2);
}

export function RegisterForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const oauthError = searchParams.get("error");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [googlePending, setGooglePending] = useState(false);

  const registerMutation = useMutation({
    mutationFn: registerWithPassword,
    onSuccess: () => {
      router.replace("/app");
    },
    onError: (error: Error) => {
      setFormError(error.message);
    },
  });

  async function handleGoogleSignUp() {
    setFormError(null);
    setGooglePending(true);

    try {
      const clientState = buildClientState();
      const redirectURI = `${window.location.origin}/auth/callback`;

      sessionStorage.setItem(OAUTH_CLIENT_STATE_STORAGE_KEY, clientState);
      const payload = await getGoogleAuthURL(redirectURI, clientState);
      window.location.assign(payload.authUrl);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Failed to start Google sign up.",
      );
      setGooglePending(false);
    }
  }

  return (
    <Card className="border-zinc-700/60">
      <CardHeader>
        <CardTitle>Create Account</CardTitle>
        <CardDescription>
          Register to start posting through the microblog backend.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            setFormError(null);
            registerMutation.mutate({ name, email, password });
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              autoComplete="name"
              placeholder="Your name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete="new-password"
              placeholder="Create a strong password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </div>
          <Button
            type="submit"
            size="lg"
            className="w-full"
            disabled={registerMutation.isPending}
          >
            {registerMutation.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <UserPlus className="h-4 w-4" />
            )}
            Create Account
          </Button>
        </form>

        <div className="relative py-1">
          <div className="absolute inset-0 flex items-center">
            <span className="w-full border-t border-zinc-800" />
          </div>
          <div className="relative flex justify-center">
            <span className="bg-zinc-950 px-3 text-xs uppercase tracking-[0.18em] text-zinc-500">
              or
            </span>
          </div>
        </div>

        <Button
          variant="outline"
          size="lg"
          className="w-full"
          onClick={handleGoogleSignUp}
          disabled={googlePending}
          type="button"
        >
          {googlePending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <span className="text-base leading-none">G</span>
          )}
          Continue with Google
        </Button>

        {(formError || oauthError) && (
          <motion.p
            initial={{ opacity: 0, y: -2 }}
            animate={{ opacity: 1, y: 0 }}
            className="rounded-md border border-rose-600/40 bg-rose-500/10 px-3 py-2 text-sm text-rose-300"
          >
            {formError || oauthError}
          </motion.p>
        )}

        <p className="text-center text-sm text-zinc-400">
          Already have an account?{" "}
          <Link href="/auth/login" className="font-semibold text-cyan-300 hover:text-cyan-200">
            Sign in
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}
