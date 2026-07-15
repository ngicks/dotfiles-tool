///|
/// The server's own dotfiles checkout and where it was cloned from.
priv struct RepoConfig {
  dir : String
  origin : String
}

///|
/// Resolve the server checkout location and its clone origin.
///
/// The checkout lives under the data dir. The origin is taken from the git
/// repository the server was launched from (its cwd); if that is not a git
/// checkout, an already-existing server clone's origin is used as a fallback.
async fn RepoConfig::resolve() -> RepoConfig raise Error {
  let dir = @os_path.join_all([
    @os_path.user_data_dir(),
    "dotfiles-server",
    "dotfiles",
  ])
  let origin = match detect_origin(@devenv.current_dir()) {
    Some(o) => o
    None =>
      match detect_origin(dir) {
        Some(o) => o
        None =>
          raise Failure::Failure(
            "could not determine dotfiles git origin; run `server serve` from a dotfiles checkout at least once",
          )
      }
  }
  { dir, origin }
}

///|
/// `git remote get-url origin` for `repo_dir`, or `None` when unavailable.
async fn detect_origin(repo_dir : String) -> String? {
  let (code, out) = @process.collect_stdout(
    "git",
    ["remote", "get-url", "origin"],
    cwd=repo_dir,
  ) catch {
    _ => return None
  }
  if code != 0 {
    return None
  }
  let url = out.text().trim()
  if url == "" {
    None
  } else {
    Some(url.to_owned())
  }
}

///|
/// Clone the checkout if it does not exist yet. No-op once present.
async fn RepoConfig::ensure_clone(self : RepoConfig) -> Unit raise Error {
  if @fs.exists(@os_path.join(self.dir, ".git")) {
    return
  }
  println("cloning \{self.origin} -> \{self.dir}")
  let parent = @os_path.dirname(self.dir)
  if !@fs.exists(parent) {
    @fs.mkdir(parent, permission=0o755, recursive=true)
  }
  let code = @process.run("git", ["clone", self.origin, self.dir])
  if code != 0 {
    raise Failure::Failure("git clone failed")
  }
}

///|
/// Fast-forward the checkout. Returns whether `HEAD` actually moved.
async fn RepoConfig::update(self : RepoConfig) -> Bool raise Error {
  let before = @devenv.git_head(self.dir)
  let code = @process.run("git", ["pull", "--ff-only"], cwd=self.dir)
  if code != 0 {
    raise Failure::Failure("git pull failed")
  }
  let after = @devenv.git_head(self.dir)
  before != after
}
