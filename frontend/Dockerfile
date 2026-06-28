# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Yarn is pre-installed in node:20-alpine (Corepack-enabled).
# Copy package files first to maximize layer cache hit on source changes.
COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

# Copy source code
COPY . .

# Build the Next.js application
RUN yarn build

# ── Production stage ──────────────────────────────────────────────────────────
FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY package.json yarn.lock ./

# Install production dependencies only
RUN yarn install --frozen-lockfile --production

# Copy built files
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public

EXPOSE 3000

CMD ["yarn", "start"]
