import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { AUTH_REFRESH_COOKIE_NAME } from "@/lib/auth/server-constants";

export default async function HomePage() {
  const cookieStore = await cookies();
  const hasRefreshCookie = Boolean(cookieStore.get(AUTH_REFRESH_COOKIE_NAME)?.value);

  if (hasRefreshCookie) {
    redirect("/app");
  }

  redirect("/auth/login");
}
