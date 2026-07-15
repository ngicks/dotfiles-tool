# dotfiles-tool

Tools for [ngicks/dotfiles](https://github.com/ngicks/dotfiles), installed
through [mise](https://mise.jdx.dev/).

Each tool is versioned independently; releases are tagged `<tool>/v<semver>`
(e.g. `dotfilesmgr/v0.0.1`, `podman-static-dist/v0.0.1`).

## Tools

- `dotfilesmgr`: moonbit daemon/CLI for dotfiles management
  (devenv build/pull REST API, daily update). Installed via
  [mise-moon-backend-plugin](https://github.com/ngicks/mise-moon-backend-plugin):

  ```
  mise plugin install moon https://github.com/ngicks/mise-moon-backend-plugin
  mise use -g 'moon:https://github.com/ngicks/dotfiles-tool#dotfilesmgr[tag_prefix=dotfilesmgr/v]@0.0.1'
  ```

- `podman-static-dist`: builds/installs
  [podman-static](https://github.com/mgoltzsche/podman-static.git) with my own
  configuration. Go module; installed via the mise go backend:

  ```
  mise use -g 'go:github.com/ngicks/dotfiles-tool/podman-static-dist/cmd/podman-static-dist@0.0.1'
  ```
