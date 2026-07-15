///|
/// How often the upstream poller checks for new commits, in milliseconds (30 min).
let poll_interval_ms = 1_800_000

///|
/// Server runtime state: the checkout it manages, whether auto-build is enabled,
/// the job registry, and the singleflight machinery.
priv struct Daemon {
  repo : RepoConfig
  auto_build : Bool
  queue : @aqueue.Queue[Job]
  cond : @cond_var.Cond
  jobs : Map[String, Job]
  inflight : Map[String, Job]
  mut next_id : Int
}

///|
fn Daemon::new(repo : RepoConfig, auto_build : Bool) -> Daemon {
  {
    repo,
    auto_build,
    queue: @aqueue.Queue::Queue(kind=@aqueue.Kind::Unbounded),
    cond: @cond_var.Cond::Cond(),
    jobs: {},
    inflight: {},
    next_id: 0,
  }
}

///|
/// Submit a job, or return the in-flight job sharing `key` (singleflight).
///
/// Registration is synchronous, so concurrent submissions with the same key
/// can never both enqueue: the second one observes the first in `inflight`.
async fn Daemon::submit(
  self : Daemon,
  kind : String,
  key : String,
  payload : JobPayload,
) -> Job {
  match self.inflight.get(key) {
    Some(job) => job
    None => {
      self.next_id = self.next_id + 1
      let job = Job::{
        id: "job-\{self.next_id}",
        kind,
        key,
        payload,
        state: Queued,
      }
      self.jobs[job.id] = job
      self.inflight[key] = job
      self.queue.put(job)
      job
    }
  }
}

///|
/// Block until `job` reaches a terminal state.
async fn Daemon::wait_for(self : Daemon, job : Job) -> Unit {
  while job.state is (Queued | Running) {
    self.cond.wait()
  }
}

///|
/// Execute one job according to its payload. Raises on failure.
async fn Daemon::run_job(self : Daemon, job : Job) -> Unit raise Error {
  match job.payload {
    BuildJob(experimental~) => {
      // An explicit build refreshes the checkout first (best-effort), then builds.
      try self.repo.update() |> ignore catch {
        e => println("warn: git pull failed: \{e}")
      }
      @devenv.build(self.repo.dir, experimental)
    }
    PullJob(remote~, experimental~) =>
      @devenv.pull(self.repo.dir, remote, experimental)
  }
}

///|
/// Serial build worker. Processing one job at a time is what makes long blocking
/// tasks singleflight: nothing else runs while a build is in progress.
async fn Daemon::worker(self : Daemon) -> Unit {
  for ;; {
    let job = self.queue.get()
    job.state = Running
    try {
      self.run_job(job)
      job.state = Succeeded
    } catch {
      e => job.state = Failed(e.to_string())
    }
    self.inflight.remove(job.key)
    self.cond.broadcast()
  }
}

///|
/// Background poller. When auto-build is enabled it periodically pulls the
/// checkout and rebuilds the devenv image on new commits. When disabled it stays
/// idle, so the server only acts on explicitly requested jobs.
async fn Daemon::poll_loop(self : Daemon) -> Unit {
  if !self.auto_build {
    println(
      "auto-build disabled (set \{auto_build_env_var}=1 in ~/.config/dotfiles-server/.env to enable); explicit jobs only",
    )
    return
  }
  for ;; {
    @async.sleep(poll_interval_ms)
    try {
      if self.repo.update() {
        println("upstream changed: auto-building devenv image")
        self.submit(
          "devenv-build",
          "devenv-build:rel",
          BuildJob(experimental=false),
        )
        |> ignore
      }
    } catch {
      e => println("poll error: \{e}")
    }
  }
}
