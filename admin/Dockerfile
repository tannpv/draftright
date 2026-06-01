# Stage 1: Build
FROM node:20-alpine AS build

# Backend the SPA talks to.  Default is prod; dev compose overrides
# via --build-arg.  Vite bakes the value into the bundle at build
# time (import.meta.env.VITE_API_URL), so rebuild is required to
# change backends.
ARG VITE_API_URL=https://api.draftright.info
ENV VITE_API_URL=$VITE_API_URL

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# Stage 2: Serve
FROM nginx:alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
