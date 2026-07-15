///|
/// Fast-forward every tracked branch of the checkout at `repo_dir`.
///
/// Mirrors deno's `git pull --all --ff-only`; stdout/stderr are inherited so the
/// pull's progress is visible at login.
async fn git_pull(repo_dir : String) -> Unit raise Error {
  let code = @process.run("git", ["pull", "--all", "--ff-only"], cwd=repo_dir)
  if code != 0 {
    raise Failure::Failure("git pull --all --ff-only failed in \{repo_dir}")
  }
}
