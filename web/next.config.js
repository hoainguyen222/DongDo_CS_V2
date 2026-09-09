/** @type {import('next').NextConfig} */
const path = require('path');

/**
 * Backend origin the Next.js proxy rewrites forward to.
 *
 * Resolution order:
 *   1. process.env.BACKEND_URL (set by docker-compose / .env files)
 *   2. Fallback to http://localhost:8080 for plain `npm run dev` on the host
 *
 * Typical values:
 *   - Local dev (no Docker):  http://localhost:8080
 *   - Inside docker-compose:  http://server:8080   (compose service name)
 *   - Kubernetes / prod:      http://api.internal:8080
 */
const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

const nextConfig = {
  reactStrictMode: false,
  // SCSS imports dùng `@/...` cần được resolve qua `tsconfig.paths`
  // vì vậy ta hook vào webpack để handle các import SCSS với alias này.
  webpack(config) {
    config.resolve.alias['@'] = path.resolve(__dirname, 'src');
    return config;
  },
  async rewrites() {
    // Note: keys in next.config.js are computed at build time. Changing
    // BACKEND_URL after `next build` requires a rebuild. For runtime
    // overrides, prefer setting BACKEND_URL in the environment that
    // invokes `next start` / `next dev`.
    return [
      {
        source: '/api/:path*',
        destination: `${BACKEND_URL}/api/:path*`,
      },
      {
        source: '/auth/:path*',
        destination: `${BACKEND_URL}/auth/:path*`,
      },
      {
        source: '/guest/:path*',
        destination: `${BACKEND_URL}/guest/:path*`,
      },
      {
        source: '/history/:path*',
        destination: `${BACKEND_URL}/history/:path*`,
      },
      {
        source: '/chat',
        destination: `${BACKEND_URL}/chat`,
      },
      {
        source: '/static/:path*',
        destination: `${BACKEND_URL}/static/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
