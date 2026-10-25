FROM node:24-slim
RUN apt-get update && apt-get install -y --no-install-recommends git git-lfs openssh-client ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN mkdir -p /daten/spiegel && chown node:node /daten/spiegel
USER node
CMD ["node", "src/main.ts"]
