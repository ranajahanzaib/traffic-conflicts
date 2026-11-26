# Dashboard

The React dashboard is built with Vite. `pnpm dev` starts the hot-reloading frontend on `http://127.0.0.1:5173`; API requests are proxied to the Go service at `http://127.0.0.1:8080`.

Run `pnpm build` to type-check and create the static production bundle in `dist/`. The Go service serves this bundle in local and production-style runs.
