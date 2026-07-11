---
title: Development
weight: 50
description: Test Metago and build or preview this documentation.
---

# Development

## Test Metago

```sh
go test ./...
UPDATE_GOLDEN=1 go test ./...
```

Golden fixtures live under `testdata/`. Set `UPDATE_GOLDEN=1` only when intentionally accepting changed generated output.

## Documentation preview

Install the Tailwind build dependencies once:

```sh
npm install --prefix docs
```

Then start Hugo from the repository root. Hugo invokes Tailwind automatically while building:

```sh
hugo server --source docs
```

Open **[http://localhost:1313](http://localhost:1313)**. Hugo watches Markdown and layout files and refreshes the browser as they change.

To include draft pages:

```sh
hugo server --source docs --buildDrafts
```

## Production build

Compile the site into the ignored root-level `dist/` directory:

```sh
hugo --source docs --destination ../dist --minify
```

Hugo cleans stale generated files in the destination by default. The output is static HTML and can be served by any static host.

## Write a page

Add Markdown under `docs/content/` with title and navigation weight:

```md
---
title: New feature
weight: 60
---

# New feature

Documentation goes here.
```

Lower weights appear earlier in the sidebar.
