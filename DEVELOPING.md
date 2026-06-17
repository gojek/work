## Web UI

```
cd cmd/workwebui
go run main.go
open "http://localhost:5040/"
```

To exercise the admin (mutating) endpoints locally:

```
WORK_ADMIN_PASSWORD=secret go run ./cmd/workwebui
```

The browser will prompt for credentials (`admin` / `secret`) the first time you click
Pause / Purge / Reset Lock / edit `max_concurrency` from the Queues page.

## Assets

Web UI frontend is written in [react](https://facebook.github.io/react/). [Webpack](https://webpack.github.io/) is used to transpile and bundle es7 and jsx to run on modern browsers.
Finally bundled js is embedded in a go file.

All NPM commands can be found in `package.json`.

- fetch dependency: `npm install`
- test: `npm test`
- generate test coverage: `npm run cover`
- lint: `npm run lint`
- bundle for production: `npm run build`
- bundle for testing: `npm run dev`

To embed bundled js, do

```
go install github.com/kevinburke/go-bindata/v4/go-bindata@latest
cd webui/internal/assets
go generate
```
