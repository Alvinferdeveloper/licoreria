# Builds with both `podman build` and `docker build`, from the repository root (pnpm workspace).
FROM docker.io/library/node:22-alpine AS build
WORKDIR /src
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile

COPY apps/web apps/web
# Next.js writes the /api/v1 proxy destination into the build.
ARG API_ORIGIN=http://api:8080
ENV API_ORIGIN=$API_ORIGIN \
    NEXT_TELEMETRY_DISABLED=1
RUN pnpm --filter web build

FROM docker.io/library/node:22-alpine
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000 \
    TZ=America/Managua

COPY --from=build --chown=node /src/apps/web/.next/standalone ./
COPY --from=build --chown=node /src/apps/web/.next/static apps/web/.next/static

USER node
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
