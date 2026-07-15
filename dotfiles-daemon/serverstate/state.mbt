///|
/// libc `getpid(2)`. Available on every supported native platform.
extern "c" fn c_getpid() -> Int = "getpid"

///|
/// The current process id.
pub fn current_pid() -> Int {
  c_getpid()
}

///|
/// Discovery information persisted by `server serve` and consumed by clients.
///
/// Kept intentionally small: it only needs to let a client (or systemd) find a
/// running server and confirm it is alive. `pid` is the server process id and
/// `url` is the base URL the server is listening on (e.g. `http://127.0.0.1:38123`).
pub struct Info {
  pid : Int
  url : String
} derive(ToJson, FromJson, Eq, Debug)

///|
pub fn Info::new(pid : Int, url : String) -> Info {
  { pid, url }
}

///|
/// Directory that holds the server state file, namespaced under the runtime base.
/// `join` tolerates a trailing separator on `XDG_RUNTIME_DIR`.
pub fn state_dir() -> String {
  @os_path.join(@os_path.user_runtime_dir(), "dotfiles-server")
}

///|
/// Absolute path of the server state file.
pub fn state_path() -> String {
  @os_path.join(state_dir(), "server-state")
}

///|
/// Persist `info` to the state file, creating the runtime directory if needed.
pub async fn write(info : Info) -> Unit raise Error {
  // `mkdir(recursive=true)` still errors if the leaf exists, and systemd's
  // RuntimeDirectory= pre-creates this dir, so only create it when absent.
  let dir = state_dir()
  if !@fs.exists(dir) {
    @fs.mkdir(dir, permission=0o700, recursive=true)
  }
  @fs.write_file(
    state_path(),
    info.to_json().stringify(),
    create_mode=@fs.CreateMode::CreateOrTruncate,
    permission=0o600,
  )
}

///|
/// Read the persisted server info, or `None` if no state file exists.
pub async fn read() -> Info? raise Error {
  let path = state_path()
  if !@fs.exists(path) {
    return None
  }
  let text = @fs.read_file(path).text()
  Some(@json.from_json(@json.parse(text)))
}

///|
/// Remove the state file if it exists. Safe to call when no server is running.
pub async fn clear() -> Unit raise Error {
  let path = state_path()
  if @fs.exists(path) {
    @fs.remove(path)
  }
}
