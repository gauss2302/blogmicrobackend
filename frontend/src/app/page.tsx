import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { mayHaveSession } from "@/lib/server/session";

export default async function HomePage() {
  if (mayHaveSession(await cookies())) {
    redirect("/app");
  }

  redirect("/auth/login");
}
