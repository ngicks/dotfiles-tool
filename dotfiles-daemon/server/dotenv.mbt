///|
/// Env var that turns on the upstream auto-build poller.
let auto_build_env_var = "DOTFILES_SERVER_AUTO_BUILD"

///|
/// Absolute path of the optional server dotenv file.
fn dotenv_path() -> String raise Error {
  @os_path.join_all([@os_path.user_config_dir(), "dotfiles-server", ".env"])
}

///|
/// Load the optional dotenv file at `~/.config/dotfiles-server/.env` into the
/// process environment.
///
/// No-op when the file is absent. Existing environment variables win, so a value
/// already exported (e.g. via a systemd `Environment=` line) is never
/// overwritten. Each line is `KEY=VALUE`; blank lines, `#` comments, an optional
/// `export ` prefix, and surrounding single/double quotes on the value are all
/// tolerated.
async fn load_dotenv() -> Unit raise Error {
  let path = dotenv_path()
  if !@fs.exists(path) {
    return
  }
  let content = @fs.read_file(path).text()
  content.split("\n").each(line => apply_dotenv_line(line.to_owned()))
}

///|
/// Apply one `KEY=VALUE` dotenv line to the process environment.
fn apply_dotenv_line(raw : String) -> Unit {
  let line = raw.trim().to_owned()
  if line.is_empty() || line.has_prefix("#") {
    return
  }
  let body = match line.strip_prefix("export ") {
    Some(rest) => rest.trim().to_owned()
    None => line
  }
  guard body.split_once("=") is Some((raw_key, raw_value)) else { return }
  let key = raw_key.trim().to_owned()
  if key.is_empty() {
    return
  }
  // Never clobber a value already present in the real environment.
  if @env.get_env_var(key) is Some(_) {
    return
  }
  @env.set_env_var(key, unquote(raw_value.trim().to_owned()))
}

///|
/// Strip a single pair of matching surrounding quotes, if present.
fn unquote(s : String) -> String {
  let n = s.length()
  if n >= 2 &&
    (
      (s.has_prefix("\"") && s.has_suffix("\"")) ||
      (s.has_prefix("'") && s.has_suffix("'"))
    ) {
    s[1:n - 1].to_owned()
  } else {
    s
  }
}

///|
/// Whether automatic devenv rebuilds are enabled.
///
/// Driven by `DOTFILES_SERVER_AUTO_BUILD` (from the real environment or the
/// dotenv file loaded by `load_dotenv`). Truthy values are `1`, `true`, `yes`,
/// `on` (case-insensitive); anything else, or unset, leaves it off.
fn auto_build_enabled() -> Bool {
  match @env.get_env_var(auto_build_env_var) {
    Some(value) =>
      match value.trim().to_owned().to_lower() {
        "1" | "true" | "yes" | "on" => true
        _ => false
      }
    None => false
  }
}
