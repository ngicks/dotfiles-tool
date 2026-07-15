///|
/// Resolve the running server's base URL from the runtime state file.
async fn server_url() -> String raise Error {
  match @serverstate.read() {
    Some(info) => info.url
    None =>
      raise Failure::Failure(
        "no running dotfiles server (state file \{@serverstate.state_path()} not found); start one with `dotfiles server serve`",
      )
  }
}

///|
/// POST a JSON body to `path` and return the parsed response body.
///
/// Raises on a non-2xx status, surfacing the server's error body.
async fn post_json(path : String, body : Json) -> Json raise Error {
  let (resp, data) = @http.post(server_url() + path, body.stringify(), headers={
    "content-type": "application/json",
  })
  let payload = data.json()
  if resp.code >= 400 {
    raise Failure::Failure(
      "\{path} failed: HTTP \{resp.code} \{resp.reason}: \{payload.stringify()}",
    )
  }
  payload
}

///|
/// GET `path` and return the parsed response body.
///
/// Raises on a non-2xx status, surfacing the server's error body.
async fn get_json(path : String) -> Json raise Error {
  let (resp, data) = @http.get(server_url() + path)
  let payload = data.json()
  if resp.code >= 400 {
    raise Failure::Failure(
      "\{path} failed: HTTP \{resp.code} \{resp.reason}: \{payload.stringify()}",
    )
  }
  payload
}

///|
/// Submit a devenv image build. Returns a ticket to query with `status`/`wait`.
pub async fn devenv_build(
  experimental : Bool,
) -> @serverstate.Ticket raise Error {
  @json.from_json(post_json("/devenv/build", { "experimental": experimental }))
}

///|
/// Submit a remote build + pull. Returns a ticket to query with `status`/`wait`.
pub async fn devenv_pull(
  remote : String,
  experimental : Bool,
) -> @serverstate.Ticket raise Error {
  @json.from_json(
    post_json("/devenv/pull", { "remote": remote, "experimental": experimental }),
  )
}

///|
/// Current status of the job identified by `ticket` (non-blocking).
pub async fn status(
  ticket : @serverstate.Ticket,
) -> @serverstate.JobStatus raise Error {
  @json.from_json(get_json("/jobs/\{ticket.id}"))
}

///|
/// Block until the job identified by `ticket` finishes, then return its result.
pub async fn wait(
  ticket : @serverstate.Ticket,
) -> @serverstate.JobStatus raise Error {
  @json.from_json(get_json("/jobs/\{ticket.id}/wait"))
}
