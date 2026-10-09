# CareFund Documentation Site

This directory contains the source files for the CareFund documentation book, rendered with [mdBook](https://rust-lang.github.io/mdBook/).

The published book is hosted on GitHub Pages at [https://healthcarefund.github.io/carefund-platform/](https://healthcarefund.github.io/carefund-platform/).

## Prerequisites

- **mdBook**: v0.5.4
- **lychee**: v0.24.2 (for link validation)

### Installing Tools

Download mdBook v0.5.4:
```bash
curl -sL "https://github.com/rust-lang/mdBook/releases/download/v0.5.4/mdbook-v0.5.4-x86_64-unknown-linux-gnu.tar.gz" | tar xz -C /usr/local/bin
```

Download lychee v0.24.2:
```bash
curl -sL "https://github.com/lycheeverse/lychee/releases/download/lychee-v0.24.2/lychee-x86_64-unknown-linux-gnu.tar.gz" | tar xz -C /tmp
sudo install -m 755 /tmp/lychee-x86_64-unknown-linux-gnu/lychee /usr/local/bin/lychee
```

## Local Build and Preview

To build the static site locally into `docs-site/book/`:
```bash
cd docs-site
mdbook build
```

To run a live local preview server on `http://localhost:3000`:
```bash
cd docs-site
mdbook serve
```

## Link Checking

Verify that all internal anchors and links resolve without broken targets:
```bash
cd docs-site
lychee --offline --root-dir book/ \
  --exclude-path book/404.html \
  --exclude-path book/print.html \
  book/
```

Note: `mdbook build` generates HTML chapters but does not validate link targets. Running `lychee` against the generated output is a mandatory CI and release validation step.

## Continuous Integration

Documentation is checked on every pull request and deployed to GitHub Pages on pushes to `main` via `.github/workflows/docs.yml`. The output directory `docs-site/book/` is git-ignored and never committed directly to the repository.
