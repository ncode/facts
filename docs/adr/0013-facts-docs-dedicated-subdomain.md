# Facts docs use a dedicated custom subdomain

Facts will publish its documentation site at `https://facts.martinez.io/` from this repository, rather than under the existing `martinez.io` personal site or the default `ncode.github.io/facts` repository Pages URL. The site should stay independently deployable from the Facts repo, with GitHub Pages serving the generated site and a `CNAME` for `facts.martinez.io`.

## Considered Options

- **Use `facts.martinez.io`** - chosen: it gives Facts a stable public URL while keeping the project docs independent from the existing personal site.
- **Use `martinez.io/facts`** - rejected: it would couple Facts documentation deployment and routing to the personal website.
- **Use `ncode.github.io/facts`** - rejected: it works as a fallback, but the custom domain is cleaner for public documentation.
