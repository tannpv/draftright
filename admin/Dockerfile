# Stage 1: Build
FROM node:20-alpine AS build

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .

# Backend the SPA talks to.  Default is prod; dev compose overrides
# via --build-arg VITE_API_URL=https://api.dev.draftright.info.  Vite
# bakes the value into the JS bundle at build time
# (import.meta.env.VITE_API_URL), so a rebuild is required to change
# backends.
#
# IMPORTANT: ARG sits IMMEDIATELY before the build step so any change
# in the value invalidates only the `npm run build` layer (npm ci
# stays cached).  The `echo` interpolates VITE_API_URL into the RUN
# command's hash — without it, two builds with different args could
# share a cache key when Docker decides the prior layer was
# equivalent.  Observed 2026-06-01 on dev: admin-dev shipped with
# api.draftright.info baked in even though compose passed
# api.dev.draftright.info.
ARG VITE_API_URL=https://api.draftright.info
ENV VITE_API_URL=$VITE_API_URL
RUN echo "Building admin SPA for backend: $VITE_API_URL" && npm run build

# Stage 2: Serve
FROM nginx:alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
