///|
/// Lifecycle state of a long-running server job.
priv enum JobState {
  Queued
  Running
  Succeeded
  Failed(String)
}

///|
/// What a job actually does, with its parameters.
priv enum JobPayload {
  BuildJob(experimental~ : Bool)
  PullJob(remote~ : String, experimental~ : Bool)
}

///|
/// A unit of long-running work tracked by the server.
///
/// `key` is the singleflight key: while a job with a given key is queued or
/// running, duplicate submissions return the same `Job` instead of enqueuing a
/// second one. `state` is mutated in place by the worker; readers (HTTP
/// handlers) observe it directly thanks to MoonBit's cooperative scheduling.
priv struct Job {
  id : String
  kind : String
  key : String
  payload : JobPayload
  mut state : JobState
}

///|
/// The job's wire-format ticket: the handle clients hold to query it later.
fn Job::ticket(self : Job) -> @serverstate.Ticket {
  { id: self.id }
}

///|
/// The job's wire-format status snapshot, suitable for an API response.
fn Job::to_status(self : Job) -> @serverstate.JobStatus {
  let (status, message) = match self.state {
    Queued => ("queued", "")
    Running => ("running", "")
    Succeeded => ("succeeded", "")
    Failed(m) => ("failed", m)
  }
  { id: self.id, kind: self.kind, status, message }
}
