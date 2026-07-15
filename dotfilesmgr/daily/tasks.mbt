///|
/// The daily-update task list, in run order.
///
/// Equivalent to deno's `tasks` array. Extend it to add steps to the daily run.
pub fn tasks() -> Array[Task] {
  [
    Task::{ name: "git pull", run: git_pull },
    Task::{ name: "sync config", run: sync_config },
  ]
}
