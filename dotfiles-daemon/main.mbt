///|
/// The `devenv` subcommands (`build`, `pull`). `with_wait` adds a `--wait`
/// flag, which only makes sense for the client (the server runs jobs async).
fn devenv_subcommands(with_wait~ : Bool) -> Array[@argparse.Command] {
  let build_flags = [
    @argparse.FlagArg("exp", short='e', about="experimental build"),
  ]
  let pull_flags = [
    @argparse.FlagArg("exp", short='e', about="experimental build"),
  ]
  if with_wait {
    build_flags.push(
      @argparse.FlagArg("wait", about="block until the job finishes"),
    )
    pull_flags.push(
      @argparse.FlagArg("wait", about="block until the job finishes"),
    )
  }
  [
    @argparse.Command(
      "build",
      about="build the devenv image",
      flags=build_flags,
    ),
    @argparse.Command(
      "pull",
      about="build on a remote and pull the resulting image",
      flags=pull_flags,
      positionals=[
        @argparse.PositionArg("remote", about="ssh target to build on"),
      ],
    ),
  ]
}

///|
/// A `devenv` command grouping the build/pull subcommands.
fn devenv_command(with_wait~ : Bool) -> @argparse.Command {
  @argparse.Command(
    "devenv",
    about="devenv image operations",
    arg_required_else_help=true,
    subcommand_required=true,
    subcommands=devenv_subcommands(with_wait~),
  )
}

///|
/// The top-level CLI: server / client / standalone.
fn cli() -> @argparse.Command {
  @argparse.Command(
    "dotfiles",
    about="dotfiles management CLI",
    arg_required_else_help=true,
    subcommand_required=true,
    subcommands=[
      @argparse.Command(
        "server",
        about="run the dotfiles server",
        arg_required_else_help=true,
        subcommand_required=true,
        subcommands=[
          @argparse.Command(
            "serve",
            about="serve the dotfiles HTTP API (REST/JSON over TCP)",
            options=[
              @argparse.OptionArg(
                "url",
                long="url",
                about="listen address, e.g. 127.0.0.1:9787 (default: 127.0.0.1:9787)",
              ),
            ],
          ),
        ],
      ),
      @argparse.Command(
        "client",
        about="request operations from a running dotfiles server",
        arg_required_else_help=true,
        subcommand_required=true,
        subcommands=[devenv_command(with_wait=true)],
      ),
      @argparse.Command(
        "standalone",
        about="run operations directly, without a server",
        arg_required_else_help=true,
        subcommand_required=true,
        subcommands=[
          devenv_command(with_wait=false),
          @argparse.Command(
            "update-daily",
            about="run the daily update (git pull + sync host config)",
          ),
        ],
      ),
    ],
  )
}

///|
fn has_flag(matches : @argparse.Matches, name : String) -> Bool {
  matches.flags.get_or_default(name, false)
}

///|
fn optional_value(matches : @argparse.Matches, name : String) -> String? {
  match matches.values.get(name) {
    Some([value, ..]) => Some(value)
    _ => None
  }
}

///|
fn required_value(
  matches : @argparse.Matches,
  name : String,
) -> String raise Error {
  match matches.values.get(name) {
    Some([value, ..]) => value
    _ => raise Failure::Failure("missing required argument: \{name}")
  }
}

///|
async fn dispatch_server(matches : @argparse.Matches) -> Unit raise Error {
  match matches.subcommand {
    Some(("serve", sub)) => {
      let url = optional_value(sub, "url")
      @server.serve(url?)
    }
    _ => raise Failure::Failure("missing server subcommand")
  }
}

///|
/// Print a submitted job's ticket, then optionally block for its result.
async fn report_job(
  ticket : @serverstate.Ticket,
  wait : Bool,
) -> Unit raise Error {
  println("job \{ticket.id} submitted")
  if wait {
    let result = @client.wait(ticket)
    println("job \{result.id}: \{result.status}")
    if result.message != "" {
      println(result.message)
    }
  }
}

///|
async fn dispatch_client(matches : @argparse.Matches) -> Unit raise Error {
  guard matches.subcommand is Some(("devenv", devenv)) else {
    raise Failure::Failure("missing client subcommand")
  }
  match devenv.subcommand {
    Some(("build", sub)) =>
      report_job(
        @client.devenv_build(has_flag(sub, "exp")),
        has_flag(sub, "wait"),
      )
    Some(("pull", sub)) =>
      report_job(
        @client.devenv_pull(required_value(sub, "remote"), has_flag(sub, "exp")),
        has_flag(sub, "wait"),
      )
    _ => raise Failure::Failure("missing devenv subcommand")
  }
}

///|
async fn dispatch_standalone(matches : @argparse.Matches) -> Unit raise Error {
  let repo_dir = @devenv.current_dir()
  match matches.subcommand {
    Some(("devenv", devenv)) =>
      match devenv.subcommand {
        Some(("build", sub)) => @devenv.build(repo_dir, has_flag(sub, "exp"))
        Some(("pull", sub)) =>
          @devenv.pull(
            repo_dir,
            required_value(sub, "remote"),
            has_flag(sub, "exp"),
          )
        _ => raise Failure::Failure("missing devenv subcommand")
      }
    Some(("update-daily", _)) => @daily.update_daily(repo_dir)
    _ => raise Failure::Failure("missing standalone subcommand")
  }
}

///|
async fn dispatch(matches : @argparse.Matches) -> Unit raise Error {
  match matches.subcommand {
    Some(("server", sub)) => dispatch_server(sub)
    Some(("client", sub)) => dispatch_client(sub)
    Some(("standalone", sub)) => dispatch_standalone(sub)
    _ => raise Failure::Failure("missing command")
  }
}

///|
async fn main {
  let matches = cli().parse() catch {
    e => {
      println(e.to_string())
      return
    }
  }
  dispatch(matches) catch {
    e => println("error: " + e.to_string())
  }
}
