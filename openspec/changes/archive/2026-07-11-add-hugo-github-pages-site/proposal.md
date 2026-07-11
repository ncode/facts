## Why

Facts has a strong README and generated reference docs, but no standalone public documentation site. A GitHub Pages site gives Facts a stable project URL at `https://facts.martinez.io/` while keeping the existing Markdown docs as the source of truth.

## What Changes

- Add a Hugo-powered documentation site for Facts.
- Publish the site with GitHub Pages from a GitHub Actions build artifact, not committed generated HTML.
- Use the existing `docs/` Markdown and generated supported-fact pages as Hugo content.
- Add a custom domain CNAME for `facts.martinez.io`.
- Style the site from the README visual language: near-white canvas, near-black ink, mono technical labels, and the existing mesh-gradient hero palette.
- Keep v1 static: no npm, no Hugo modules, no theme dependency, no client-side interactivity.

## Capabilities

### New Capabilities

- `facts-documentation-site`: Public Hugo documentation site for Facts, including homepage, docs navigation, custom domain, and GitHub Pages deployment.

### Modified Capabilities

(none)

## Impact

- **Docs/site**: root Hugo configuration, custom layouts, CSS, static assets, CNAME, and docs content metadata or index pages as needed.
- **CI/deployment**: GitHub Actions workflow for building Hugo and publishing to GitHub Pages.
- **Existing docs**: `docs/supported-facts/*.md` remain generated from `docs/schema/facts.yaml`; Hugo renders them but does not own their content.
- **Product behavior**: No change to fact resolution, CLI behavior, library API, schema contract, or release target support.
