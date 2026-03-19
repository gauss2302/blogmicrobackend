import { Suspense } from "react";

import { CallbackExchange } from "@/components/auth/callback-exchange";

export default function OAuthCallbackPage() {
  return (
    <Suspense fallback={null}>
      <CallbackExchange />
    </Suspense>
  );
}
