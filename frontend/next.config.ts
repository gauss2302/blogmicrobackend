import type { NextConfig } from "next";
import path from "node:path";

// The Content-Security-Policy is set per-request in middleware.ts so it can carry
// a script nonce (removing 'unsafe-inline'). The remaining headers are static and
// apply to every response.
const securityHeaders = [
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Frame-Options", value: "DENY" },
];

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  outputFileTracingRoot: path.join(process.cwd()),
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // The per-request CSP (middleware.ts) intentionally skips /api. BFF
      // responses are JSON and never a document/resource context, so lock them
      // to a no-op policy here as defense-in-depth alongside nosniff.
      {
        source: "/api/:path*",
        headers: [
          {
            key: "Content-Security-Policy",
            value: "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
          },
        ],
      },
    ];
  },
};

export default nextConfig;
