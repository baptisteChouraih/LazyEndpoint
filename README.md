# LazyEndpoint

Browse an OpenAPI (Swagger) specification in your terminal — the same way
[`lazygit`](https://github.com/jesseduffield/lazygit) or
[`lazydocker`](https://github.com/jesseduffield/lazydocker) do it for git and
docker, but for web APIs.

```
 ┌───────────────────────┐╭──────────────────────────────────────────────╮
 │ 5 endpoints           ││ 1 Overview  2 Parameters  3 Body  4 Responses│
 │ ▾ PETS                ││ GET /pets/{petId}                            │
 │  GET   /pets          ││                                              │
 │  POST  /pets          ││ Get a pet by id                              │
 │  GET   /pets/{petId}  ││                                              │
 │  DELETE /pets/{petId} ││ Tags        pets                             │
 │ ▾ STORE               ││ Operation   getPetById                       │
 │  GET   /store/invent… ││ Servers     https://petstore.example.com/v1  │
 │ ▾ USERS               ││                                              │
 │  GET   /users/{usern… ││ EXAMPLE REQUEST                              │
 └───────────────────────┘╰──────────────────────────────────────────────╯
 j/k move · / filter · tab pane · 1-4 tab · ? help · q quit
```

## Features

- Reads OpenAPI 3.x documents from a file (YAML or JSON) or an http(s) URL,
  resolving internal and external `$ref`s.
- Endpoints grouped by tag, in the order the spec declares its tags.
- Fuzzy-ish filtering by method, path, summary, operation id or tag (`/`).
- Detail pane with four tabs: summary and example request, parameters,
  request body schema, and responses with their schemas.
- Schemas are rendered as readable, indented outlines instead of raw JSON.
- **Try it out**: press `t` to send the selected request for real — edit the
  server, the path/query/header parameters and the JSON body, then press `s`.
- Adaptive colors for light and dark terminals.

## Install

```bash
go install github.com/baptisteChouraih/LazyEndpoint@latest
```

Or build from a checkout:

```bash
git clone https://github.com/baptisteChouraih/LazyEndpoint
cd LazyEndpoint
go build -o lazyendpoint .
```

## Usage

```bash
lazyendpoint examples/petstore.yaml
lazyendpoint openapi.json
lazyendpoint https://petstore3.swagger.io/api/v3/openapi.json
lazyendpoint api.yaml -tag pets
```

| Flag       | Description                          |
| ---------- | ------------------------------------ |
| `-tag`     | only show endpoints with that tag    |
| `-version` | print the version and exit           |

## Keys

| Key            | Action                                    |
| -------------- | ----------------------------------------- |
| `j` / `k`, `↑` / `↓` | move the cursor, scroll the details |
| `tab`          | switch the focused pane                   |
| `1` - `4`      | Overview · Parameters · Body · Responses   |
| `[` / `]`      | previous / next detail tab                |
| `/`            | filter endpoints                          |
| `esc`          | clear the filter, focus the list          |
| `t`            | open / close the request tester           |
| `g` / `G`      | top / bottom of the details               |
| `ctrl+d`/`ctrl+u` | half page down / up                    |
| `?`            | help                                      |
| `q`            | quit                                      |

## Try it out

Press `t` on an endpoint to open the tester. Values are prefilled from the
spec: the first server URL, parameter `default`/`enum` values, and a JSON body
generated from the request schema.

| Key     | Action                                  |
| ------- | --------------------------------------- |
| `j`/`k` | select a field, scroll the response     |
| `enter` | edit the selected field (`esc` cancels) |
| `s`     | send the request                        |
| `tab`   | switch between request and response     |
| `esc`   | close the tester                        |

Requests time out after 30 seconds and response bodies are capped at 1 MB.
JSON responses are pretty printed; status, duration, size and headers are
shown. `s` performs a real HTTP request, so it only goes where you point it in
the `server` field.

`securitySchemes` (bearer tokens, API keys) are not applied yet, so endpoints
that require them need the header added as a parameter in the spec.

## Status

Working reader with a request tester. It answers: what does this API expose,
what does each call take and return, and what happens when I call it?

Ideas for next steps:

- Apply `securitySchemes` when sending requests.
- Remember per-endpoint request values between runs.
- Copy the curl command or a response example to the clipboard.
- Search across schemas, jump to a `$ref` definition.
- Compare two versions of a spec and show what changed.

## Development

```bash
go build ./...
go test ./...
```

Layout:

- `internal/spec` — loads the OpenAPI document and flattens it into the
  display model (`Document`, `Group`, `Endpoint`).
- `internal/tui` — the Bubble Tea model, key handling, layout and renderers.

## License

GPL-3.0, see [LICENSE](LICENSE).
