import type { NextConfig } from "next";

/** Where the Go API listens; the browser only talks to this app, which proxies /api/v1. */
const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  // Backups and restores outlast the default 30 s of the /api/v1 proxy.
  experimental: { proxyTimeout: 300_000 },
  cacheComponents: true,
  partialPrefetching: true,
  async rewrites() {
    return [{ source: "/api/v1/:path*", destination: `${apiOrigin}/api/v1/:path*` }];
  },
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
