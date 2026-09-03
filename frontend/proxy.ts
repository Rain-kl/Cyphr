import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';

/**
 * Next.js 16 代理层
 *
 * 1. API 请求速率限制
 * 2. 页面身份验证
 */

/* ==================== 速率限制 ==================== */

interface RateLimitEntry {
  count: number;
  windowStart: number;
}

const rateLimitStore = new Map<string, RateLimitEntry>();

/** 不需要速率限制的路径 */
const EXCLUDED_PREFIXES = ['/api/v1/config', '/epay/', '/lpay/'];

/** 速率限制规则: [最大请求数, 窗口时长(ms)] */
const RATE_LIMITS: Record<string, [number, number]> = {
  '/api/v1/oauth/login': [1, 5000],
  '/api/v1/oauth/callback': [1, 5000],
};

/** 默认限制: 60次/60秒 */
const DEFAULT_RATE_LIMIT: [number, number] = [60, 60000];

function getRateLimit(pathname: string): [number, number] {
  if (RATE_LIMITS[pathname]) return RATE_LIMITS[pathname];

  const match = Object.keys(RATE_LIMITS)
    .filter((prefix) => pathname.startsWith(prefix))
    .sort((a, b) => b.length - a.length)[0];

  return match ? RATE_LIMITS[match] : DEFAULT_RATE_LIMIT;
}

function shouldRateLimit(pathname: string): boolean {
  return !EXCLUDED_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

function checkRateLimit(
  identifier: string,
  pathname: string,
): [boolean, number] {
  const [maxRequests, windowMs] = getRateLimit(pathname);
  const key = `${identifier}:${pathname}`;
  const now = Date.now();
  const entry = rateLimitStore.get(key);

  if (!entry || now - entry.windowStart >= windowMs) {
    rateLimitStore.set(key, { count: 1, windowStart: now });
    return [true, 0];
  }

  entry.count++;
  if (entry.count > maxRequests) {
    return [false, Math.ceil((windowMs - (now - entry.windowStart)) / 1000)];
  }
  return [true, 0];
}

if (typeof setInterval !== 'undefined') {
  setInterval(() => {
    const now = Date.now();
    for (const [key, entry] of rateLimitStore.entries()) {
      if (now - entry.windowStart > 120000) rateLimitStore.delete(key);
    }
  }, 60000);
}

function backendOrigin(): string {
  return process.env.WAVELET_BACKEND_URL || 'http://localhost:8000';
}

function isMultipart(request: NextRequest): boolean {
  return (request.headers.get('content-type') || '').includes(
    'multipart/form-data',
  );
}

function isDocumentNavigation(request: NextRequest): boolean {
  return request.headers.get('sec-fetch-dest') === 'document';
}

async function proxyApiToBackend(request: NextRequest): Promise<NextResponse> {
  const { pathname, search } = request.nextUrl;
  const url = `${backendOrigin()}${pathname}${search}`;
  const headers = new Headers();
  request.headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (
      lower === 'host' ||
      lower === 'connection' ||
      lower === 'content-length' ||
      lower === 'transfer-encoding'
    ) {
      return;
    }
    headers.set(key, value);
  });
  const host = request.headers.get('host');
  if (host) {
    headers.set('x-forwarded-host', host);
  }
  headers.set(
    'x-forwarded-proto',
    request.nextUrl.protocol.replace(':', '') || 'http',
  );

  const method = request.method.toUpperCase();
  const init: RequestInit = {
    method,
    headers,
    redirect: 'manual',
  };
  if (method !== 'GET' && method !== 'HEAD') {
    init.body = request.body;
    Object.assign(init, { duplex: 'half' });
  }

  const upstream = await fetch(url, init);

  if (isDocumentNavigation(request)) {
    if (upstream.status === 401) {
      const loginUrl = new URL('/login', request.url);
      loginUrl.searchParams.set('callbackUrl', '/asr');
      return NextResponse.redirect(loginUrl);
    }
    if (upstream.status === 403) {
      return NextResponse.redirect(new URL('/403', request.url));
    }
  }

  const responseHeaders = new Headers();
  upstream.headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (lower === 'set-cookie' || lower === 'transfer-encoding') {
      return;
    }
    responseHeaders.set(key, value);
  });
  const out = new NextResponse(upstream.body, {
    status: upstream.status,
    headers: responseHeaders,
  });
  const cookies =
    typeof upstream.headers.getSetCookie === 'function'
      ? upstream.headers.getSetCookie()
      : [];
  for (const cookie of cookies) {
    out.headers.append('set-cookie', cookie);
  }
  return out;
}

/* ==================== 代理主函数 ==================== */

export async function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const sessionCookieName =
    process.env.WAVELET_SESSION_COOKIE_NAME || 'wavelet_session_id';
  const sessionCookie = request.cookies.get(sessionCookieName);

  // WebSocket upgrades must pass through to rewrites untouched.
  if (request.headers.get('upgrade')?.toLowerCase() === 'websocket') {
    return NextResponse.next();
  }

  /* API 请求：速率限制后反代，并原样回写 Set-Cookie（rewrites 会丢登录 Cookie） */
  if (pathname.startsWith('/api/')) {
    const rateLimitEnabled = process.env.WAVELET_RATE_LIMIT_ENABLED === 'true';

    if (rateLimitEnabled && shouldRateLimit(pathname)) {
      const identifier =
        sessionCookie?.value ||
        request.headers.get('x-forwarded-for')?.split(',')[0].trim() ||
        request.headers.get('x-real-ip') ||
        'anonymous';

      const [allowed, waitTime] = checkRateLimit(identifier, pathname);
      if (!allowed) {
        return NextResponse.json(
          {
            error_code: 'RATE_LIMITED',
            error_msg: `请求过于频繁，请 ${waitTime} 秒后重试`,
          },
          { status: 429, headers: { 'Retry-After': String(waitTime) } },
        );
      }
    }
    if (isMultipart(request)) {
      return NextResponse.next();
    }
    try {
      return await proxyApiToBackend(request);
    } catch {
      return NextResponse.json(
        { error_msg: '无法连接到服务器', data: null },
        { status: 502 },
      );
    }
  }

  /* 页面请求：公共路由放行 */
  const publicRoutes = [
    '/',
    '/login',
    '/register',
    '/callback',
    '/privacy',
    '/terms',
    '/icon',
    '/403',
  ];
  const publicPrefixes = ['/docs/', '/epay/'];

  if (
    publicRoutes.includes(pathname) ||
    publicPrefixes.some((p) => pathname.startsWith(p))
  ) {
    return NextResponse.next();
  }

  if (!sessionCookie) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('callbackUrl', pathname + search);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    '/api/((?!v1/admin/logs/ws).*)',
    '/((?!_next|favicon.ico|robots.txt|sitemap.xml|api/v1/admin/logs/ws|.*\\.(?:jpg|jpeg|gif|png|svg|ico|webp)).*)',
  ],
};
