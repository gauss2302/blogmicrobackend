import type { Metadata } from "next";
import { headers } from "next/headers";

import { AppProviders } from "@/components/providers/app-providers";
import { ThemeScript } from "@/components/theme/theme-script";
import { ThemeProvider } from "@/components/theme/theme-provider";

import "./globals.css";

export const metadata: Metadata = {
  title: "Microblog Frontend",
  description: "Secure Next.js frontend for the microblog gRPC backend.",
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // The CSP nonce is set per-request by middleware; the inline theme script must
  // carry it now that 'unsafe-inline' has been removed from script-src.
  const nonce = (await headers()).get("x-nonce") ?? undefined;

  return (
    <html lang="en" suppressHydrationWarning>
      <body className="antialiased">
        <ThemeScript nonce={nonce} />
        <AppProviders>
          <ThemeProvider>{children}</ThemeProvider>
        </AppProviders>
      </body>
    </html>
  );
}
