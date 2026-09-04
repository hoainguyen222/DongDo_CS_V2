import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// Routes that require admin authentication
const protectedRoutes = ['/admin'];

// Routes under /admin that are PUBLIC (no auth required)
const adminPublicRoutes = ['/admin/login'];

// Routes that should redirect authenticated admin users away
// Mỗi route có target redirect riêng:
//   - /admin/login → /admin/inbox (vào admin dashboard)
//   - /login        → /            (về portal khách hàng) — CHỈ khi có dongdo_auth_token
// NOTE: /login bây giờ là form guest (họ tên + phone), không phải admin login
const authRoutes: Array<{ prefix: string; redirect: string }> = [
  { prefix: '/admin/login', redirect: '/admin/inbox' },
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

  // If accessing protected admin route without auth, redirect to /admin/login
  if (isProtectedRoute && !isAuthenticated) {
    return NextResponse.redirect(new URL('/admin/login', request.url));
  }

  // If already authenticated and trying to access /admin/login, redirect to /admin/inbox
  if (pathname === '/admin/login' && isAuthenticated) {
    return NextResponse.redirect(new URL('/admin/inbox', request.url));
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
