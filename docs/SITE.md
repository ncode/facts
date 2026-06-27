# Documentation Site

Facts publishes a Hugo documentation site at `https://facts.martinez.io/`.

Build it from the repository root:

```sh
hugo --cleanDestinationDir --minify
```

The site uses `docs/` as Hugo content, writes generated HTML to `public/`, and
publishes that directory through GitHub Pages. Do not commit `public/`.
