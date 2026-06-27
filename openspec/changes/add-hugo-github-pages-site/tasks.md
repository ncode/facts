## 1. Hugo Site Setup

- [x] 1.1 Add root Hugo configuration with `baseURL = "https://facts.martinez.io/"`, `docs/` as the content source, and no theme.
- [x] 1.2 Add `static/CNAME` containing `facts.martinez.io`.
- [x] 1.3 Ensure Hugo generated output such as `public/` is ignored and not committed.
- [x] 1.4 Document the local Hugo build command in the site or repository docs.

## 2. Layouts And Styling

- [x] 2.1 Add minimal custom Hugo layouts for the homepage, single docs pages, docs lists, and shared page chrome.
- [x] 2.2 Add local CSS using the README palette, system sans/mono font stacks, restrained cards, hairline borders, and responsive spacing.
- [x] 2.3 Build the homepage hero with Facts as the first-viewport signal and a large README-color mesh-gradient treatment.
- [x] 2.4 Add static terminal/code blocks for `go get github.com/ncode/facts`, `brew install ncode/tap/facts`, and representative `facts` CLI usage.

## 3. Documentation Content And Navigation

- [x] 3.1 Add shallow navigation groups for Start, Library, CLI, Supported facts, and Project.
- [x] 3.2 Map existing `docs/` Markdown pages into those navigation groups without duplicating their content into a second docs tree.
- [x] 3.3 Render `docs/supported-facts/*.md` as supported-fact reference pages while leaving their content owned by the schema generator.
- [x] 3.4 Keep non-page docs assets and schema files out of primary navigation unless explicitly linked.

## 4. GitHub Pages Deployment

- [x] 4.1 Add a GitHub Actions workflow that builds Hugo on the default branch and `workflow_dispatch`.
- [x] 4.2 Publish the generated `public/` directory through the GitHub Pages artifact deployment flow.
- [x] 4.3 Configure the workflow permissions and concurrency needed for GitHub Pages deployment.

## 5. Verification

- [x] 5.1 Run the local Hugo build and confirm it includes the homepage, docs pages, supported-fact pages, and `CNAME`.
- [x] 5.2 Inspect the generated homepage and at least one supported-fact page for broken links or missing styling.
- [x] 5.3 Run `go test ./...` to catch any generated supported-fact documentation drift.
- [x] 5.4 Run `go vet ./...` before handoff.
