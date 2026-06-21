/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone', // 启用 standalone 模式用于 Docker 部署
  
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096',
  },
  
  async rewrites() {
    // 开发模式下代理 API 请求到 Go 服务器
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096'}/api/:path*`,
      },
      {
        source: '/static/:path*',
        destination: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096'}/static/:path*`,
      },
    ];
  },

  // 图片配置
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: '**.bilibili.com',
      },
      {
        protocol: 'https',
        hostname: '**.hdslb.com',
      },
      {
        protocol: 'https',
        hostname: '**.ytimg.com',
      },
      {
        protocol: 'https',
        hostname: '**.youtube.com',
      },
    ],
  },
};

module.exports = nextConfig;
