import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: {
    root: __dirname,
  },
  cacheComponents: true,
  partialPrefetching: true,
};

export default nextConfig;
