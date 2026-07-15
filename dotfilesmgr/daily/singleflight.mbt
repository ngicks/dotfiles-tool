///|
/// Directory holding the update-daily lock file.
///
/// Prefers the conventional per-user runtime dir (`$XDG_RUNTIME_DIR/dotfiles`)
/// and falls back to the marker dir when it is unset (darwin, WSL without
/// systemd). Deliberately not `@os_path.user_runtime_dir()`: its `/tmp`
/// fallback is shared across users, which is wrong for a per-user lock.
fn lock_dir() -> String raise Error {
  match @env.get_env_var("XDG_RUNTIME_DIR") {
    Some(dir) if dir != "" => @os_path.join(dir, "dotfiles")
    _ => marker_dir()
  }
}

///|
/// Open (creating if missing) the update-daily lock file.
///
/// The file is never deleted: removing a flock'd path is racy (a second
/// process can lock a recreated file on a different inode), and an empty
/// leftover file is harmless.
async fn open_lock_file() -> @fs.File raise Error {
  let dir = lock_dir()
  if !@fs.exists(dir) {
    @fs.mkdir(dir, permission=0o755, recursive=true)
  }
  @fs.open(
    @os_path.join(dir, "update-daily.lock"),
    mode=ReadWrite,
    create_mode=OpenOrCreate,
    permission=0o644,
  )
}

///|
/// Whether the marker's mtime moved forward from `before` to `after`.
fn marker_advanced(before : (Int64, Int)?, after : (Int64, Int)?) -> Bool {
  match (before, after) {
    (_, None) => false
    (None, Some(_)) => true
    (Some((b_sec, b_ns)), Some((a_sec, a_ns))) =>
      a_sec > b_sec || (a_sec == b_sec && a_ns > b_ns)
  }
}

///|
/// Run `run` at most once across concurrent processes (file-lock singleflight).
///
/// Concurrent invocations serialize on an exclusive flock, and the interval
/// marker's mtime is the shared "result": if it advanced while we were
/// acquiring the lock, another process already completed the update and we
/// skip. If we had to wait and the marker did not advance, the process we
/// waited on failed — that failure is shared (raised here, marker untouched)
/// instead of every queued login retrying in turn; the next login retries.
///
/// flock is released automatically when the process dies, so a crashed winner
/// never leaves a stale lock behind.
async fn singleflight(run : async () -> Unit) -> Unit raise Error {
  let before = marker_mtime()
  let file = open_lock_file()
  defer file.close()

  let waited = if file.try_lock(Exclusive) {
    false
  } else {
    println("another update-daily is running; waiting for it to finish")
    file.lock(Exclusive)
    true
  }
  // Closing the fd releases the flock on its own; unlock anyway for the sake
  // of being explicit. `File::unlock` swallows errors, so it is defer-safe.
  defer file.unlock()

  if marker_advanced(before, marker_mtime()) {
    println("daily update already completed by another process; skipping")
    return
  }

  if waited {
    raise Failure::Failure(
      "concurrent update-daily failed; leaving retry to the next login",
    )
  }
  run()
}
