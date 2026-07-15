///|
/// One `./config` subtree to mirror onto the Windows host.
priv struct SyncPair {
  name : String
  target : String
}

///|
/// Config subtrees synced to the Windows host profile under WSL.
///
/// Extend this list to mirror more host-side GUI app configs.
let sync_pairs : Array[SyncPair] = [
  { name: "wezterm", target: ".config/wezterm" },
]

///|
/// Whether we're running under WSL.
///
/// Detected like the deno version: a `WSLInterop*` handler registered in
/// `/proc/sys/fs/binfmt_misc`. That directory is absent on non-Linux hosts (and
/// on Linux without WSL), so a missing/unreadable directory simply means "no
/// host-side sync needed".
async fn is_wsl() -> Bool {
  let entries = @fs.readdir("/proc/sys/fs/binfmt_misc") catch {
    _ => return false
  }
  for name in entries {
    if name.has_prefix("WSLInterop") {
      return true
    }
  }
  false
}

///|
/// WSL mount path of the Windows user profile (`%USERPROFILE%`).
///
/// Reads `$env:USERPROFILE` via `powershell.exe` (e.g. `C:\Users\me`) and
/// rewrites it to the `/mnt/c/Users/me` form WSL exposes.
async fn host_profile_dir() -> String raise Error {
  let (code, out) = @process.collect_stdout("powershell.exe", [
    "$env:USERPROFILE",
  ])
  if code != 0 {
    raise Failure::Failure("powershell.exe failed to resolve USERPROFILE")
  }
  let profile = out.text().trim().to_owned()
  guard profile.split_once(":") is Some((drive, rest)) else {
    raise Failure::Failure("unexpected USERPROFILE format: \{profile}")
  }
  "/mnt/" +
  drive.to_owned().to_lower() +
  rest.to_owned().replace_all(old="\\", new="/")
}

///|
/// Streaming SHA-256 digest of the file at `path`.
///
/// Reads the file in chunks through its `@io.Reader` and folds each chunk into
/// the hash context, so the file's full contents never sit in memory at once.
async fn file_hash(path : String) -> FixedArray[Byte] raise Error {
  let file = @fs.open(path, mode=@fs.Mode::ReadOnly)
  defer file.close()
  let ctx = @crypto.SHA256::new()
  while file.read_some() is Some(chunk) {
    ctx.update(chunk)
  }
  ctx.finalize()
}

///|
/// Stream-copy the file at `src` onto `dst` (truncating), feeding `dst`'s writer
/// straight from `src`'s reader so neither file is buffered in memory.
async fn copy_file(src : String, dst : String) -> Unit raise Error {
  let reader = @fs.open(src, mode=@fs.Mode::ReadOnly)
  defer reader.close()
  let writer = @fs.open(
    dst,
    mode=@fs.Mode::WriteOnly,
    create_mode=@fs.CreateMode::CreateOrTruncate,
    permission=0o644,
  )
  defer writer.close()
  writer.write_reader(reader)
}

///|
/// Recursively copy `src` onto `dst`.
///
/// Directories are created as needed; a regular file is copied only when its
/// content hash differs from the destination, so identical files are skipped
/// entirely (no rewrite, no mtime churn). Both the hash and the copy stream the
/// file in chunks instead of reading it wholesale into memory.
async fn copy_tree(src : String, dst : String) -> Unit raise Error {
  if @fs.kind(src) is @fs.FileKind::Directory {
    if !@fs.exists(dst) {
      @fs.mkdir(dst, permission=0o755, recursive=true)
    }
    for name in @fs.readdir(src) {
      copy_tree(@os_path.join(src, name), @os_path.join(dst, name))
    }
    return
  }
  if @fs.exists(dst) &&
    @fs.kind(dst) is @fs.FileKind::Regular &&
    file_hash(src) == file_hash(dst) {
    return
  }
  copy_file(src, dst)
  println("synced \{dst}")
}

///|
/// Mirror selected `./config` subtrees onto the Windows host (WSL only).
///
/// Mirrors deno's `syncConfig`: under WSL, copy `config/<name>` into the Windows
/// user profile so host-side GUI apps (wezterm) pick up the dotfiles. On any
/// other host this is a no-op.
async fn sync_config(repo_dir : String) -> Unit raise Error {
  if !is_wsl() {
    return
  }
  let host = host_profile_dir()
  for pair in sync_pairs {
    let src = @os_path.join_all([repo_dir, "config", pair.name])
    let dst = @os_path.join(host, pair.target)
    println("sync \{src} -> \{dst}")
    copy_tree(src, dst)
  }
}
