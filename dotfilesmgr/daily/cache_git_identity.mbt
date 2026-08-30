///|
/// Directory holding the cached git identity.
///
/// Shares `cache_dir()` rather than re-deriving `XDG_CACHE_HOME`, so this path
/// agrees with the login script and the devenv bind mount by construction.
fn git_identity_dir() -> String raise Error {
  @os_path.join(cache_dir(), "git")
}

///|
/// `git config --global --get <key>`, or `None` when the key is unset.
///
/// `--global` reads only the user-level files (`~/.gitconfig` or
/// `$XDG_CONFIG_HOME/git/config`): the cached identity represents the user, so
/// a repository-local identity must not leak into it.
/// Every failure mode collapses to `None` -- a spawn error (no git on PATH), the
/// exit status git uses for "unset", and an empty value -- because the caller
/// treats a missing identity as "nothing to cache", not as an error.
async fn git_config_value(key : String) -> String? {
  let (code, out) = @process.collect_stdout("git", [
    "config", "--global", "--get", key,
  ]) catch {
    _ => return None
  }
  if code != 0 {
    return None
  }
  let value = out.text().trim()
  if value == "" {
    None
  } else {
    Some(value.to_owned())
  }
}

///|
/// Cache the git identity into `<cache>/dotfiles/git/config`.
///
/// The devenv container bind-mounts that directory onto `/root/.config/git`, so
/// git inside the container commits as the host user without the identity being
/// baked into the image.
///
/// A missing identity is a warning, not an error: raising here would stop
/// `run_tasks` from stamping the interval marker, and every login on a machine
/// without a configured identity would then re-run the whole daily update. A
/// failing write still raises, since that is a real fault worth retrying.
async fn cache_git_identity(_repo_dir : String) -> Unit raise Error {
  guard git_config_value("user.name") is Some(name) else {
    println("git identity not cached: user.name is unset")
    return
  }
  guard git_config_value("user.email") is Some(email) else {
    println("git identity not cached: user.email is unset")
    return
  }
  let dir = git_identity_dir()
  if !@fs.exists(dir) {
    @fs.mkdir(dir, permission=0o755, recursive=true)
  }
  let path = @os_path.join(dir, "config")
  @fs.write_file(
    path,
    "[user]\n\tname = \{name}\n\temail = \{email}\n",
    create_mode=@fs.CreateMode::CreateOrTruncate,
    permission=0o644,
  )
  println("cached git identity \{name} <\{email}> -> \{path}")
}
