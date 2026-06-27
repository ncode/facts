## Context

Facts already has a polished README, generated supported-fact Markdown, and reference docs under `docs/`, but it does not publish a standalone website. The agreed direction is a Hugo site published from this repository to `https://facts.martinez.io/`, independent from the existing `martinez.io` personal site.

The current repository has no JavaScript site pipeline. The site should therefore add only Hugo source and a GitHub Pages workflow, keeping generated HTML out of git and keeping `docs/schema/facts.yaml` plus `tools/supportedfacts` as the owners of supported-fact reference content.

## Goals / Non-Goals

**Goals:**

- Publish a static Hugo documentation site for Facts at `https://facts.martinez.io/`.
- Reuse existing Markdown documentation under `docs/` instead of duplicating docs into a second tree.
- Build and deploy with GitHub Actions using the Pages artifact flow.
- Match the README visual language and the Vercel-inspired reference: near-white surfaces, near-black ink, mono technical labels, restrained cards, and a hero-scale mesh gradient.
- Keep v1 dependency-light: no npm, no theme dependency, no client-side JavaScript requirement.

**Non-Goals:**

- No docs search, version switcher, analytics, or interactive terminal in v1.
- No committed generated HTML.
- No change to fact schema generation, fact resolution, CLI output, or library API.
- No merge with the existing `martinez.io` website.

## Decisions

- **Use Hugo with custom layouts.** Hugo renders the existing Markdown and gives GitHub Pages a plain static artifact. A third-party theme is rejected because it would add dependency churn and fight the README-derived design.
- **Keep Hugo source at the repository root and `docs/` as content.** Root-level `hugo.toml`, `layouts/`, and `static/` keep the site obvious while avoiding a duplicate `site/content` copy of the docs.
- **Use `facts.martinez.io` as the canonical base URL.** This follows ADR-0013 and keeps Facts documentation independent from the personal site. `static/CNAME` owns the Pages custom-domain file.
- **Publish from GitHub Actions artifacts.** The workflow builds Hugo into `public/` and deploys with the GitHub Pages artifact actions. Generated HTML stays out of git.
- **Let the schema generator own supported-fact pages.** Hugo only renders `docs/supported-facts/*.md`; the Go generator and existing tests remain responsible for keeping them in sync with `docs/schema/facts.yaml`.
- **Use system fonts and local CSS only.** The README SVG already uses platform sans and mono stacks. External font packages, npm, and client-side scripts are rejected for v1.

## Risks / Trade-offs

- **Markdown without front matter may render with weak titles** -> derive titles in Hugo templates from headings or file names, and add front matter only to hand-authored docs if needed.
- **`docs/` contains non-page assets and generated files** -> configure Hugo/layouts so only intended Markdown pages appear in navigation, while static assets are served from `static/` or copied deliberately.
- **GitHub Pages custom domain needs DNS outside the repo** -> commit `static/CNAME` and document that DNS for `facts.martinez.io` must point at the Pages host.
- **No theme means fewer built-in docs features** -> acceptable for v1; add search or deeper navigation only when docs volume requires it.
