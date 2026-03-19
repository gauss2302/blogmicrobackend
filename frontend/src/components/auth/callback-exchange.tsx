"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { motion } from "framer-motion";
import { Loader2, ShieldCheck, ShieldX } from "lucide-react";

import {
  exchangeGoogleAuthCode,
} from "@/lib/auth/client-api";
import { OAUTH_CLIENT_STATE_STORAGE_KEY } from "@/lib/auth/client-constants";

type CallbackState = "pending" | "success" | "error";

export function CallbackExchange() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [state, setState] = useState<CallbackState>("pending");
  const [message, setMessage] = useState("Exchanging secure authorization code...");

  const authCode = searchParams.get("auth_code");
  const callbackState = searchParams.get("state");
  const oauthError = searchParams.get("error");

  const hasValidCode = useMemo(() => Boolean(authCode && authCode.trim().length > 0), [authCode]);

  useEffect(() => {
    let active = true;

    async function run() {
      if (oauthError) {
        setState("error");
        setMessage(oauthError);
        return;
      }

      if (!hasValidCode || !authCode) {
        setState("error");
        setMessage("Missing auth code from OAuth callback.");
        return;
      }

      const storedState = sessionStorage.getItem(OAUTH_CLIENT_STATE_STORAGE_KEY);
      if (storedState && callbackState && storedState !== callbackState) {
        setState("error");
        setMessage("OAuth state mismatch. Please try again.");
        return;
      }

      try {
        await exchangeGoogleAuthCode({ authCode });
        if (!active) {
          return;
        }

        setState("success");
        setMessage("Authentication complete. Redirecting...");
        sessionStorage.removeItem(OAUTH_CLIENT_STATE_STORAGE_KEY);
        router.replace("/app");
      } catch (error) {
        if (!active) {
          return;
        }

        setState("error");
        setMessage(
          error instanceof Error ? error.message : "OAuth exchange failed.",
        );
      }
    }

    run();

    return () => {
      active = false;
    };
  }, [authCode, callbackState, hasValidCode, oauthError, router]);

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2 }}
      className="mx-auto flex min-h-screen w-full max-w-md flex-col items-center justify-center px-4 text-center"
    >
      <div className="mb-6 rounded-full border border-zinc-700 bg-zinc-900 p-4">
        {state === "pending" && <Loader2 className="h-8 w-8 animate-spin text-cyan-300" />}
        {state === "success" && <ShieldCheck className="h-8 w-8 text-emerald-300" />}
        {state === "error" && <ShieldX className="h-8 w-8 text-rose-300" />}
      </div>
      <h1 className="mb-2 text-2xl font-bold text-zinc-50">
        {state === "pending" && "Verifying Session"}
        {state === "success" && "Access Granted"}
        {state === "error" && "Authentication Failed"}
      </h1>
      <p className="max-w-sm text-sm text-zinc-400">{message}</p>
    </motion.div>
  );
}
