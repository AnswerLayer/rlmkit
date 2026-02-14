# Releases

Releases are automated with GoReleaser and GitHub Actions.

## How It Works

- CI runs on:
  - pushes to `main`
  - pushes of tags matching `v*`
- On `v*` tags, CI runs GoReleaser to build binaries and publish a GitHub Release.

Config:
- GoReleaser: `.goreleaser.yaml`
- GitHub Actions: `.github/workflows/ci.yml`

## Cutting a Release

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release job will:
- build `rlmkit` for darwin/linux/windows (amd64/arm64)
- attach archives + `checksums.txt`

