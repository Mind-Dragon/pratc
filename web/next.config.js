/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  serverExternalPackages: ['react-force-graph-2d'],
  webpack: (config) => {
    config.resolve.fallback = { 
      ...config.resolve.fallback, 
      fs: false,
      path: false,
      stream: false
    };
    return config;
  }
};

module.exports = nextConfig;
