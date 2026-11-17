import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: {
    // This nested checkout must resolve files from the frontend rather than an ancestor lockfile.
    root: __dirname,
  },
  cacheComponents: true,
  partialPrefetching: true,
};

export default nextConfig;
