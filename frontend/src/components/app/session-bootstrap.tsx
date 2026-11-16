"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";

import { restoreSession } from "@/lib/auth/client-api";
import { useAuthStore } from "@/lib/stores/auth-store";

export function SessionBootstrap({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const didBootstrap = useRef(false);
  const [isBootstrapping, setIsBootstrapping] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const runBootstrap = useCallback(async () => {
    if (useAuthStore.getState().isAuthenticated) {
      setError(null);
      setIsBootstrapping(false);
      return;
    }
    try {
      const session = await restoreSession();
      if (!session) {
        router.replace("/auth/login?error=session_expired");
        return;
      }
      setError(null);
      setIsBootstrapping(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not restore the session.");
      setIsBootstrapping(false);
    }
  }, [router]);

  useEffect(() => {
    let active = true;

    async function bootstrap() {
      if (didBootstrap.current) {
        if (active) {
          setIsBootstrapping(false);
        }
        return;
      }
      didBootstrap.current = true;
      await runBootstrap();
    }

    bootstrap();

    return () => {
      active = false;
    };
  }, [isAuthenticated, runBootstrap]);

  useEffect(() => {
    function handlePageshow(event: PageTransitionEvent) {
      if (event.persisted) {
        didBootstrap.current = false;
        setIsBootstrapping(true);
        runBootstrap();
      }
    }

    function handleVisibilityChange() {
      if (document.visibilityState === "visible" && !useAuthStore.getState().isAuthenticated) {
        didBootstrap.current = false;
        setIsBootstrapping(true);
        runBootstrap();
      }
    }

    window.addEventListener("pageshow", handlePageshow);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.removeEventListener("pageshow", handlePageshow);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [runBootstrap]);

  if (error) {
    return (
      <div className="mx-auto flex min-h-screen w-full max-w-md flex-col items-center justify-center gap-3 px-4 text-center">
        <p className="text-sm text-rose-300">{error}</p>
        <button
          type="button"
          className="text-sm text-cyan-300 underline"
          onClick={() => {
            didBootstrap.current = false;
            setError(null);
            setIsBootstrapping(true);
            void runBootstrap();
          }}
        >
          Try again
        </button>
      </div>
    );
  }

  if (isBootstrapping) {
    return (
      <div className="mx-auto flex min-h-screen w-full max-w-md flex-col items-center justify-center gap-3 px-4 text-center">
        <Loader2 className="h-6 w-6 animate-spin text-cyan-300" />
        <p className="text-sm text-zinc-400">Verifying secure session...</p>
      </div>
    );
  }

  return <>{children}</>;
}
