///|
/// Opaque handle to a submitted server job.
///
/// Returned by the client's submit calls (`devenv_build`/`devenv_pull`) and
/// accepted by `status`/`wait` to retrieve the job's result later.
pub(all) struct Ticket {
  id : String
} derive(ToJson, FromJson, Eq)

///|
/// Snapshot of a server job's lifecycle.
///
/// `status` is one of `queued`, `running`, `succeeded`, `failed`; `message`
/// carries the failure reason when `status` is `failed`, otherwise empty.
pub(all) struct JobStatus {
  id : String
  kind : String
  status : String
  message : String
} derive(ToJson, FromJson, Eq)

///|
/// Whether the job has reached a terminal (finished) state.
pub fn JobStatus::is_terminal(self : JobStatus) -> Bool {
  self.status == "succeeded" || self.status == "failed"
}

///|
/// Whether the job finished successfully.
pub fn JobStatus::is_success(self : JobStatus) -> Bool {
  self.status == "succeeded"
}
