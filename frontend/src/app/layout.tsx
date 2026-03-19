import type { Metadata } from "next";

import { AppProviders } from "@/components/providers/app-providers";
import { ThemeScript } from "@/components/theme/theme-script";
import { ThemeProvider } from "@/components/theme/theme-provider";

import "./globals.css";

export const metadata: Metadata = {
  title: "Microblog Frontend",
  description: "Secure Next.js frontend for the microblog gRPC backend.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="antialiased">
        <ThemeScript />
        <AppProviders>
          <ThemeProvider>{children}</ThemeProvider>
        </AppProviders>
      </body>
    </html>
  );
}
