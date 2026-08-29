/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
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
