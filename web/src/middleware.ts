import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// Routes that require authentication
const protectedRoutes = ['/admin'];

// Routes under /admin that are PUBLIC (no auth required)
const adminPublicRoutes = ['/admin/login'];

// Routes that should redirect authenticated users away
// Mỗi route có target redirect riêng:
//   - /admin/login → /admin/inbox (vào admin dashboard)
//   - /login        → /            (về portal khách hàng)
// Trước đây cả hai đều redirect về /admin/inbox — sai cho khách hàng.
const authRoutes: Array<{ prefix: string; redirect: string }> = [
  { prefix: '/admin/login', redirect: '/admin/inbox' },
  { prefix: '/login', redirect: '/' },
];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const authToken = request.cookies.get('dongdo_auth_token')?.value;
  const isAuthenticated = !!authToken;

  const isAdminPublic = adminPublicRoutes.some(
    (p) => pathname === p || pathname.startsWith(p + '/')
  );
  const isProtectedRoute =
    protectedRoutes.some((route) => pathname.startsWith(route)) && !isAdminPublic;

  // If accessing protected route without auth, redirect to /admin/login
  if (isProtectedRoute && !isAuthenticated) {
    return NextResponse.redirect(new URL('/admin/login', request.url));
  }

  // If already authenticated and trying to access auth routes, redirect to the
  // route-specific target. Dùng `pathname === prefix || pathname.startsWith(prefix + '/')`
  // để khớp chính xác, tránh '/login-extra' bị match nhầm bởi prefix '/login'.
  const matchedAuthRoute = authRoutes.find(
    (route) => pathname === route.prefix || pathname.startsWith(route.prefix + '/')
  );
  if (matchedAuthRoute && isAuthenticated) {
    return NextResponse.redirect(new URL(matchedAuthRoute.redirect, request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - api routes (API calls go to backend)
     * - _next/static (static files)
     * - _next/image (image optimization)
     * - favicon.ico (favicon)
     * - public files (images, fonts, etc.)
     * - . files (like .well-known)
     */
    '/((?!api|_next/static|_next/image|favicon.ico|.*\\..*|_next).*)',
  ],
};
