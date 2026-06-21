/** @type {import('next').NextConfig} */
const nextConfig = {
  // 静态导出配置 - 用于嵌入到Go项目
  output: 'export',
  distDir: '../internal/out',
  trailingSlash: true,
  
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096',
  },

  // 图片优化配置（静态导出时需要禁用）
  images: {
    unoptimized: true,
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
  
  // 注意：静态导出时不支持 rewrites、redirects、headers 等服务端功能
  // API 请求已改为使用绝对路径，支持embed静态部署
}

module.exports = nextConfig