/** @type {import('next').NextConfig} */
const path = require('path');

// Resolve the API host at runtime. In Docker compose the server service is
// reachable on the compose network as `http://server:8080`. Outside of
// Docker (dev), it falls back to localhost:8080.
// NEXT_PUBLIC_API_BASE is set in docker-compose.yml; NEXT_PUBLIC_API_BASE
// is also exposed to the browser so client-side fetches work too.
const apiBase = (process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080')
  .replace(/\/$/, '');

const nextConfig = {
  // Standalone output bundles the Next.js server with only the files it
  // actually needs at runtime, giving us a ~120MB image instead of the
  // multi-gigabyte node_modules copy.
  output: 'standalone',
  reactStrictMode: false,
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
        destination: `${apiBase}/api/:path*`,
      },
      {
        source: '/auth/:path*',
        destination: `${apiBase}/auth/:path*`,
      },
      {
        source: '/guest/:path*',
        destination: `${apiBase}/guest/:path*`,
      },
      {
        source: '/history/:path*',
        destination: `${apiBase}/history/:path*`,
      },
      {
        source: '/chat',
        destination: `${apiBase}/chat`,
      },
      {
        source: '/static/:path*',
        destination: `${apiBase}/static/:path*`,
      },
      // NOTE: WebSocket connections do NOT go through Next.js rewrites.
      // The browser opens the WS directly against the backend host
      // (see web/src/lib/ws.ts which builds `ws://<host>:8080/ws?...`).
      // Next.js only allows http/https schemes in rewrites, so we
      // intentionally leave /ws out of this list. In production behind a
      // reverse proxy (nginx/Caddy), forward `/ws` to the backend there.
    ];
  },
};

module.exports = nextConfig;
