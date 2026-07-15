///|
/// repository name for devenv images
pub let repository = "localhost/devenv/devenv"

///|
/// Fully-qualified `repository:tag` reference for a devenv image.
pub fn repo_tag(tag : String) -> String {
  repository + ":" + tag
}

///|
/// The current `HEAD` commit of the repository at `repo_dir`.
pub async fn git_head(repo_dir : String) -> String raise Error {
  let (exit_code, stdout) = @process.collect_stdout(
    "git",
    ["rev-parse", "HEAD"],
    cwd=repo_dir,
  )
  if exit_code != 0 {
    raise Failure::Failure("git rev-parse HEAD failed in \{repo_dir}")
  }
  stdout.text().trim().to_owned()
}

///|
/// The latest git tag of the repository at `repo_dir`, with any leading `v` stripped.
pub async fn git_tag(repo_dir : String) -> String raise Error {
  let (exit_code, stdout) = @process.collect_stdout(
    "git",
    ["describe", "--tags", "--abbrev=0"],
    cwd=repo_dir,
  )
  if exit_code != 0 {
    raise Failure::Failure(
      "git describe --tags --abbrev=0 failed in \{repo_dir}",
    )
  }
  let tag = stdout.text().trim()
  if tag.has_prefix("v") {
    tag[1:].to_owned()
  } else {
    tag.to_owned()
  }
}

///|
/// Image tag for a build of the repository at `repo_dir`.
///
/// Experimental builds get an `-exp1` suffix so they never clobber a release tag.
pub async fn get_tag(
  repo_dir : String,
  is_experimental : Bool,
) -> String raise Error {
  let ver = git_tag(repo_dir)
  if is_experimental {
    ver + "-exp1"
  } else {
    ver
  }
}

///|
/// Whether an image with version tag `ver` already exists in local storage.
pub async fn is_already_built(ver : String) -> Bool {
  let exit_code = @process.run("podman", ["image", "inspect", repo_tag(ver)])
  exit_code == 0
}

///|
/// Build the devenv image for an explicit version tag from `repo_dir`.
pub async fn build_ver(
  repo_dir : String,
  ver : String,
  is_experimental : Bool,
) -> Unit raise Error {
  let git_tag_val = if is_experimental { "exp" } else { ver }
  let args = [
    "buildx",
    "build",
    ".",
    "--build-arg",
    "GIT_TAG=" + git_tag_val,
    "-f",
    "./devenv/Containerfile",
    "-t",
    repo_tag(ver),
    "--no-cache",
  ]
  // proxy env vars
  match @env.get_env_var("HTTP_PROXY") {
    Some(_) => {
      let proxy_vars : Array[String] = [
        "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
      ]
      for v in proxy_vars {
        let val = @env.get_env_var(v).unwrap_or("")
        args..push("--build-arg").push(v + "=" + val)
      }
    }
    None => ()
  }
  // SSL cert secret
  args..push("--secret").push("id=cert,src=" + ca_file_path())
  let exit_code = @process.run("podman", args, cwd=repo_dir)
  if exit_code != 0 {
    raise Failure::Failure("podman build failed")
  }
}

///|
/// Build the devenv image from the repository at `repo_dir`.
///
/// No-op when the resolved version tag is already present in local storage.
pub async fn build(
  repo_dir : String,
  is_experimental : Bool,
) -> Unit raise Error {
  let ver = get_tag(repo_dir, is_experimental)
  println("building " + repo_tag(ver))
  if is_already_built(ver) {
    println("already built")
    return
  }
  build_ver(repo_dir, ver, is_experimental)
}
