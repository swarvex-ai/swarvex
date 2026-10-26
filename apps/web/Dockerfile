# Website (SvelteKit, Node-Adapter). Baukontext ist die Repo-Wurzel, weil die Doku Dateien aus
# lead/, docs/ und media/logo/ einbindet (vorgerendert beim Bauen).
FROM node:24-slim AS bau
WORKDIR /repo/apps/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY lead /repo/lead
COPY docs /repo/docs
COPY media/logo/export /repo/media/logo/export
COPY apps/web /repo/apps/web
RUN npm run build && npm prune --omit=dev

FROM node:24-slim
WORKDIR /app
COPY --from=bau /repo/apps/web/build ./build
COPY --from=bau /repo/apps/web/node_modules ./node_modules
COPY --from=bau /repo/apps/web/package.json ./
ENV NODE_ENV=production PORT=3000
USER node
CMD ["node", "build"]
