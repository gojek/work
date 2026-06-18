## Web UI

```
cd cmd/workwebui
go run main.go
open "http://localhost:5040/"
```

To exercise the optional HTTP Basic Auth locally (protects the whole UI):

```
WORK_WEBUI_AUTH_PASSWORD=secret go run ./cmd/workwebui
```

The browser prompts for credentials (`admin` / `secret`) on first load.

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
