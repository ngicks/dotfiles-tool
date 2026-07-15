///|
/// Build the REST/JSON application: a mocket route table over the daemon.
fn build_app(daemon : Daemon, info : @serverstate.Info) -> @mocket.Mocket {
  let app = @mocket.new()
  app.get("/healthz", _event => {
    json_response(200, { "status": "ok", "pid": info.pid, "url": info.url })
  })
  app.get("/status", _event => {
    json_response(200, {
      "pid": info.pid,
      "url": info.url,
      "repo": daemon.repo.dir,
      "origin": daemon.repo.origin,
      "auto_build": daemon.auto_build,
      "jobs": daemon.jobs.length(),
    })
  })
  app.get("/jobs/:id", event => {
    let id = event.params.get("id").map(v => v.to_owned()).unwrap_or("")
    match daemon.jobs.get(id) {
      Some(job) => json_response(200, job.to_status().to_json())
      None => json_response(404, { "error": "job not found", "id": id })
    }
  })
  app.get("/jobs/:id/wait", event => {
    let id = event.params.get("id").map(v => v.to_owned()).unwrap_or("")
    match daemon.jobs.get(id) {
      Some(job) =>
        try {
          daemon.wait_for(job)
          json_response(200, job.to_status().to_json())
        } catch {
          e => json_response(500, { "error": e.to_string() })
        }
      None => json_response(404, { "error": "job not found", "id": id })
    }
  })
  app.post("/devenv/build", event => {
    try {
      let body = read_json_body(event)
      let exp = json_bool_field(body, "experimental")
      let key = if exp { "devenv-build:exp" } else { "devenv-build:rel" }
      let job = daemon.submit("devenv-build", key, BuildJob(experimental=exp))
      json_response(200, job.ticket().to_json())
    } catch {
      e => json_response(500, { "error": e.to_string() })
    }
  })
  app.post("/devenv/pull", event => {
    try {
      let body = read_json_body(event)
      match json_string_field(body, "remote") {
        None => json_response(400, { "error": "remote is required" })
        Some(remote) => {
          let exp = json_bool_field(body, "experimental")
          let job = daemon.submit(
            "devenv-pull",
            "devenv-pull:\{remote}",
            PullJob(remote~, experimental=exp),
          )
          json_response(200, job.ticket().to_json())
        }
      }
    } catch {
      e => json_response(500, { "error": e.to_string() })
    }
  })
  app
}

///|
/// A JSON response with an explicit status code, as a mocket `&Responder`.
fn json_response(status : Int, body : Json) -> &@mocket.Responder {
  @mocket.HttpResponse::new(@mocket.StatusCode::from_int(status))
  .json(body)
  .to_responder()
}

///|
/// Parse the request body as JSON, defaulting to an empty object when absent
/// or malformed so handlers can treat every field as optional.
fn read_json_body(event : @mocket.MocketEvent) -> Json {
  event.req.json() catch {
    _ => Json::object({})
  }
}

///|
/// Read a boolean field from a JSON request body. Missing => false.
fn json_bool_field(body : Json, field : String) -> Bool {
  match body {
    Object(o) => o.get(field) is Some(Json::True)
    _ => false
  }
}

///|
/// Read a string field from a JSON request body. Missing => None.
fn json_string_field(body : Json, field : String) -> String? {
  match body {
    Object(o) =>
      match o.get(field) {
        Some(String(s)) => Some(s)
        _ => None
      }
    _ => None
  }
}
