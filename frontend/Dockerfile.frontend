# --- Build stage ---
FROM node:20-alpine AS build
WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

# VITE_API_URL must be supplied as a Docker BUILD ARG, not just a runtime env var,
# because Vite bakes import.meta.env.VITE_API_URL into the bundle at build time.
# In Render: Dashboard -> your frontend service -> Environment -> add it under
# "Build Arguments" (or "Docker Build Args"), not just "Environment Variables".
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

# --- Run stage: serve the static build ---
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html

# SPA fallback so client-side routes like /poll/:id don't 404 on refresh
RUN printf 'server {\n\
    listen 80;\n\
    root /usr/share/nginx/html;\n\
    index index.html;\n\
    location / {\n\
        try_files $uri /index.html;\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
