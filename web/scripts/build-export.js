#!/usr/bin/env node

const { execSync } = require('child_process');

console.log('📦 开始静态导出构建...\n');

try {
  // 设置环境变量并执行构建
  console.log('1️⃣  执行 Next.js 静态导出构建...\n');
  
  execSync('EXPORT_MODE=true next build', { 
    stdio: 'inherit',
    env: { ...process.env, EXPORT_MODE: 'true' }
  });

  console.log('\n✅ 静态导出构建完成！');
  console.log('📁 输出目录: out/\n');

} catch (error) {
  console.error('\n❌ 构建失败:', error.message);
  process.exit(1);
}

console.log('🎉 完成！\n');
