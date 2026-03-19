# Frontend (Next.js + Bun)

Secure Next.js (App Router + TypeScript) frontend for the microblog backend with:

- Next.js BFF proxy layer (`/api/bff/*`)
- HttpOnly refresh token cookie flow
- In-memory access token via Zustand (no localStorage/sessionStorage token persistence)
- Silent refresh on `401`
- Route protection via Next middleware
- ShadCN-styled UI components + Framer Motion interactions
- React Query for async state and caching

## Local Run

1. Install dependencies:
```bash
bun install
```

2. Configure environment:
```bash
cp .env.example .env.local
```

3. Start development server:
```bash
bun run dev
```

4. Open [http://localhost:3000](http://localhost:3000)

## Required Backend Settings

For cookie-based auth to work correctly, API Gateway/Auth service should be configured with:

- `AUTH_REFRESH_TOKEN_COOKIE=true`
- `AUTH_REFRESH_TOKEN_COOKIE_NAME=refresh_token` (or match frontend `AUTH_REFRESH_COOKIE_NAME`)
- `AUTH_COOKIE_DOMAIN` empty for localhost development
- `SameSite=Lax` (already requested)

## Environment Variables

- `BACKEND_API_URL` - API gateway base URL (default: `http://localhost:8080`)
- `AUTH_REFRESH_COOKIE_NAME` - refresh token cookie name (default: `refresh_token`)
- `NEXT_PUBLIC_APP_NAME` - app title

## Scripts

- `bun run dev` - dev server
- `bun run build` - production build
- `bun run start` - start production server
- `bun run lint` - ESLint
- `bun run typecheck` - TypeScript checks

## Docker (Self-host)

Build image:
```bash
docker build -t microblog-frontend:latest .
```

Run container:
```bash
docker run --rm -p 3000:3000 \
  -e BACKEND_API_URL=http://host.docker.internal:8080 \
  -e AUTH_REFRESH_COOKIE_NAME=refresh_token \
  microblog-frontend:latest
```

## Security Notes

- Access token is only in Zustand memory and cleared on refresh failure/logout.
- Refresh token is expected only in HttpOnly cookie; BFF routes forward upstream `Set-Cookie`.
- Auth pages redirect authenticated users to `/app`; protected pages redirect guests to `/auth/login`.
- OAuth callback verifies `client_state` against browser session storage before exchanging code.
