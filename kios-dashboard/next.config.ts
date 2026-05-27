import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  eslint: {
    // Lint is run separately; don't block production builds on it.
    ignoreDuringBuilds: true,
  },
  images: {
    remotePatterns: [
      // Telegram profile photos returned by the Login Widget.
      { protocol: "https", hostname: "t.me" },
      { protocol: "https", hostname: "*.telegram.org" },
      { protocol: "https", hostname: "cdn*.telesco.pe" },
    ],
  },
};

export default nextConfig;
