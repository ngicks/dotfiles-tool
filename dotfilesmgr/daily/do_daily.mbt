///|
/// A single daily-update step.
///
/// Mirrors deno's `{ name, task }` shape so the task list stays extensible: add
/// a `Task` to `tasks()` and it runs as part of the daily update. `run` receives
/// the dotfiles checkout directory.
pub struct Task {
  name : String
  run : async (String) -> Unit
}

///|
/// Run every task in order, isolating failures.
///
/// Like deno's `doDaily`, one task failing does not stop the rest: every error
/// is collected and surfaced together, which lets the caller skip stamping the
/// interval marker (leaving the next login to retry).
async fn run_tasks(repo_dir : String, tasks : Array[Task]) -> Unit raise Error {
  let errs : Array[String] = []
  for task in tasks {
    (task.run)(repo_dir) catch {
      e => {
        println("task \{task.name} failed: \{e}")
        errs.push(task.name + ": " + e.to_string())
      }
    }
  }
  if errs.length() > 0 {
    raise Failure::Failure("daily update failed: " + errs.join("; "))
  }
}
