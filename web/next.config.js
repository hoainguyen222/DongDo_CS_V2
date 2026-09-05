/** @type {import('next').NextConfig} */
const path = require('path');

const nextConfig = {
  reactStrictMode: false,
  output: 'standalone', // required for the Docker multi-stage build (web/Dockerfile)
  // SCSS imports dùng `@/...` cần được resolve qua `tsconfig.paths`
  // vì vậy ta hook vào webpack để handle các import SCSS với alias này.
  webpack(config) {
    config.resolve.alias['@'] = path.resolve(__dirname, 'src');
    return config;
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/api/:path*',
      },
      {
        source: '/auth/:path*',
        destination: 'http://localhost:8080/auth/:path*',
      },
      {
        source: '/guest/:path*',
        destination: 'http://localhost:8080/guest/:path*',
      },
      {
        source: '/history/:path*',
        destination: 'http://localhost:8080/history/:path*',
      },
      {
        source: '/chat',
        destination: 'http://localhost:8080/chat',
      },
      {
        source: '/static/:path*',
        destination: 'http://localhost:8080/static/:path*',
      },
    ];
  },
};

module.exports = nextConfig;
