/** @type {import('next').NextConfig} */
const nextConfig = {
  // 静态导出配置 - 用于嵌入到Go项目
  output: 'export',
  distDir: '../internal/server/out',
  trailingSlash: true,
  
  // 环境变量配置
  // NEXT_PUBLIC_API_URL 仅用于 next dev 跨域访问 Go 后端。
  // Go embed / 同源部署时不要设置它，前端会在运行时自动使用当前 origin。


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

  // TypeScript 配置（生产构建时忽略类型错误以加快构建速度）
  typescript: {
    // 在生产构建时忽略类型错误（开发时仍会检查）
    ignoreBuildErrors: process.env.NODE_ENV === 'production',
  },

  // ESLint 配置
  eslint: {
    // 生产构建时忽略 ESLint 错误
    ignoreDuringBuilds: process.env.NODE_ENV === 'production',
  },
  
  // 注意：静态导出时不支持 rewrites、redirects、headers 等服务端功能
  // API 请求已改为使用相对路径，支持embed静态部署
}

module.exports = nextConfig