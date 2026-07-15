///|
/// Run `command` on `remote` over SSH.
async fn ssh_exec(remote : String, command : String) -> Unit raise Error {
  run_checked("ssh", [remote, command])
}

///|
/// Build the shell command that drives the dotfiles CLI on a remote host.
///
/// Sources the login shell so the mise shims are on PATH, enters the
/// checkout, and runs the mise-managed CLI directly.
fn remote_build_command(subargs : Array[String]) -> String {
  let parts = [
    ". ~/.zshrc",
    "cd ~/.dotfiles",
    "dotfiles-daemon " + subargs.join(" "),
  ]
  parts.join(" && ")
}

///|
/// Build the devenv image on `remote` and copy it into local container storage.
///
/// The build is submitted to the remote's own daemon over SSH (so it persists if
/// this SSH session is interrupted), and once it finishes `oci-image-copy`
/// streams the resulting image directly between the two hosts' container stores
/// over SSH (skopeo + sftp), avoiding any intermediate tarball.
///
/// Requires a `dotfiles server serve` running on `remote`.
pub async fn pull(
  repo_dir : String,
  remote : String,
  is_experimental : Bool,
) -> Unit raise Error {
  let tag = get_tag(repo_dir, is_experimental)
  let image = repo_tag(tag)
  // Drive the remote build through that host's own daemon (`client`), never
  // `standalone`: the build then runs in the remote's persistent server and
  // survives this SSH session dropping. `--wait` blocks the session until the
  // daemon finishes so the image exists before we copy it; if the connection is
  // interrupted the remote keeps building, and re-running `pull` reuses the
  // in-flight (or already-built) image instead of starting over.
  let build_args = ["client", "devenv", "build", "--wait"]
  if is_experimental {
    build_args.push("--exp")
  }
  println("triggering remote build on \{remote}")
  ssh_exec(remote, remote_build_command(build_args))
  println("copying \{image} from \{remote} via oci-image-copy")
  run_checked("oci-image-copy", [
    "pull",
    image,
    "--remote",
    "ssh://" + remote,
    "--local",
    "containers-storage:",
  ])
  println("pulled \{image}")
}
