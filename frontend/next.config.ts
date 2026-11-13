import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  experimental: {
    // /api の中継は既定で30秒で打ち切られる。月間プラン生成と代替種目相談はAIの再試行を含めて
    // バックエンドの WriteTimeout（2分）まで応答しうるため、中継の上限をそこへそろえる。
    // ブラウザ側の上限は lib/api.ts の apiTimeouts で、これより少し長くしている。
    proxyTimeout: 120_000,
  },
  async rewrites() {
    const backendInternalUrl = process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";
    return [
      {
        source: "/api/:path*",
        destination: `${backendInternalUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
