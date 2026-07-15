///|
/// Run a command, raising when it exits non-zero.
async fn run_checked(
  cmd : String,
  args : Array[String],
  cwd? : String,
  extra_env? : Map[String, String],
) -> Unit raise Error {
  let cwd : StringView? = cwd.map(fn(s) { s[:] })
  let exit_code = @process.run(cmd, args, extra_env?, cwd?)
  if exit_code != 0 {
    raise Failure::Failure("\{cmd} failed with exit code \{exit_code}")
  }
}

///|
/// Absolute path of the current working directory.
///
/// Standalone commands operate on the dotfiles checkout the user invoked them
/// from, so the CLI resolves `repo_dir` to this.
pub fn current_dir() -> String raise Error {
  match @env.current_dir() {
    Some(cwd) => cwd
    None => raise Failure::Failure("current working directory is unavailable")
  }
}
