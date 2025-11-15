FROM node:20-alpine AS builder

WORKDIR /app

# Install yarn
RUN npm install -g yarn

# Copy package files
COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

# Copy source code
COPY . .

# Build the application
RUN yarn build

# Production stage
FROM node:20-alpine

WORKDIR /app

# Install yarn
RUN npm install -g yarn

# Copy package files
COPY package.json yarn.lock ./

# Install production dependencies
RUN yarn install --frozen-lockfile --production

# Copy built files
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public

EXPOSE 3000

CMD ["yarn", "start"]

