///|
fn get_env_var_or_raise(name : String) -> String raise Error {
  match @env.get_env_var(name) {
    Some(value) => value
    None => raise Failure::Failure("env var \{name} is not defined")
  }
}

///|
/// Join two path components with the platform separator, like `filepath.Join`.
/// Tolerates a trailing separator on `base` (no doubled separator).
pub fn join(base : String, segment : String) -> String {
  @path.Path(base).join(@path.Path(segment)).to_string()
}

///|
/// Join a sequence of path components left to right.
pub fn join_all(parts : Array[String]) -> String {
  guard parts is [first, .. rest] else { return "" }
  let mut acc = @path.Path(first)
  for seg in rest {
    acc = acc.join(@path.Path(seg))
  }
  acc.to_string()
}

///|
/// The directory portion of `p` (everything before the last separator).
pub fn dirname(p : String) -> String {
  @path.Path(p).dirname().to_string()
}

///|
#cfg(target="windows")
pub fn user_home_dir() -> String raise Error {
  get_env_var_or_raise("USERPROFILE")
}

///|
#cfg(not(target="windows"))
pub fn user_home_dir() -> String raise Error {
  get_env_var_or_raise("HOME")
}

///|
#cfg(target="windows")
pub fn user_config_dir() -> String raise Error {
  get_env_var_or_raise("AppData")
}

///|
#cfg(target="darwin")
pub fn user_config_dir() -> String raise Error {
  join_all([user_home_dir(), "Library", "Application Support"])
}

///|
#cfg(not(any(target="windows", target="darwin")))
pub fn user_config_dir() -> String raise Error {
  @env.get_env_var("XDG_CONFIG_HOME").unwrap_or(
    join(user_home_dir(), ".config"),
  )
}

///|
#cfg(target="windows")
pub fn user_cache_dir() -> String raise Error {
  get_env_var_or_raise("LocalAppData")
}

///|
#cfg(target="darwin")
pub fn user_cache_dir() -> String raise Error {
  join_all([user_home_dir(), "Library", "Caches"])
}

///|
#cfg(not(any(target="windows", target="darwin")))
pub fn user_cache_dir() -> String raise Error {
  @env.get_env_var("XDG_CACHE_HOME").unwrap_or(join(user_home_dir(), ".cache"))
}

///|
#cfg(target="windows")
pub fn user_data_dir() -> String raise Error {
  get_env_var_or_raise("LocalAppData")
}

///|
#cfg(target="darwin")
pub fn user_data_dir() -> String raise Error {
  join_all([user_home_dir(), "Library", "Application Support"])
}

///|
#cfg(not(any(target="windows", target="darwin")))
pub fn user_data_dir() -> String raise Error {
  @env.get_env_var("XDG_DATA_HOME").unwrap_or(
    join_all([user_home_dir(), ".local", "share"]),
  )
}

///|
/// Per-user runtime base directory for ephemeral state (sockets, pid/url files).
///
/// Returns `XDG_RUNTIME_DIR` when present, otherwise `/tmp`. Like the other
/// `user_*_dir` helpers this is the *base*; callers append their own app
/// namespace (e.g. `os_path.join(user_runtime_dir(), "dotfiles-server")`).
pub fn user_runtime_dir() -> String {
  @env.get_env_var("XDG_RUNTIME_DIR").unwrap_or("/tmp")
}
