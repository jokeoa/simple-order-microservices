# Order Frontend

React + TypeScript frontend for the order service.

## Local development

```bash
npm install
npm run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies `/api/*` to the local Go order service on `http://localhost:8080`.

## Production container

The root `docker-compose.yml` builds this app into an Nginx container and exposes it on `http://localhost:3000`. In the container, `/api/*` is proxied to `order-service`.
