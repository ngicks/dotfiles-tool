///|
/// Default listen address when `--url` is omitted.
///
/// mocket derives the port from the address string (its native server can't
/// report an OS-assigned ephemeral port back), so we bind a fixed loopback port
/// rather than `:0`. Clients still discover the exact URL from the state file.
let default_address = "127.0.0.1:9787"

///|
/// Serve the dotfiles server.
///
/// Binds the HTTP listener (`--url` or the fixed loopback default), records
/// pid+url to the runtime state file so clients can find it, starts the build
/// worker and the upstream poller, then serves until cancelled.
pub async fn serve(url? : String) -> Unit raise Error {
  @async.with_task_group(group => {
    load_dotenv()
    let repo = RepoConfig::resolve()
    repo.ensure_clone()
    let auto_build = auto_build_enabled()
    let daemon = Daemon::new(repo, auto_build)
    // Daemon tasks live for the server's lifetime; no_wait so they are
    // cancelled (not awaited) if `listen` ever returns.
    group.spawn_bg(() => daemon.worker(), no_wait=true)
    group.spawn_bg(() => daemon.poll_loop(), no_wait=true)
    let address = parse_listen(url)
    let info = @serverstate.Info::new(
      @serverstate.current_pid(),
      "http://\{address}",
    )
    @serverstate.write(info)
    group.add_defer(() => @serverstate.clear() catch { _ => () })
    let app = build_app(daemon, info)
    println("dotfiles server listening on http://\{address}")
    println("  repo:          \{repo.dir}")
    println("  origin:        \{repo.origin}")
    println("  auto-build:    \{auto_build}")
    app.listen(address)
  })
}

///|
/// Resolve the listen spec into a `host:port` address string.
///
/// Accepts `host:port`, `:port` (loopback assumed), optionally prefixed with
/// `http://` or `tcp://`. Defaults to the fixed loopback address.
fn parse_listen(url : String?) -> String {
  match url {
    None => default_address
    Some(raw) => {
      let s = strip_scheme(raw)
      if s.has_prefix(":") {
        "127.0.0.1" + s
      } else {
        s
      }
    }
  }
}

///|
/// Strip a leading `http://` or `tcp://` scheme if present.
fn strip_scheme(s : String) -> String {
  if s.has_prefix("http://") {
    s[7:].to_owned()
  } else if s.has_prefix("tcp://") {
    s[6:].to_owned()
  } else {
    s
  }
}
