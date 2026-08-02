# Follow-up Caveats

- Worker pool now starts once at process startup. Later runs enqueue jobs into the existing channel.
- Tracker status becomes `synced` when jobs are enqueued, not when downloads and post-processing finish.
- `Runner.running` becomes false when parsing and enqueueing finish, so a new run can overlap with downloads from an older run.
- Re-triggering the same tracker while its previous jobs are active can enqueue duplicate work and cause file races.
- Worker download errors are currently ignored, so `synced` does not guarantee every download succeeded.
- Parsing or database errors can leave a tracker stuck at `syncing`.
- Dev mode workers intentionally exit after one job, so they are not persistent in dev mode.
- Worker shutdown does not currently observe `context.Context`; workers remain blocked until process exit.

## Future Fix

- Add tracker identity to each queued job.
- Have workers send completion events to a coordinator.
- Track queued and active jobs per tracker.
- Mark a tracker `synced` only after job production is complete and all jobs finish successfully.
- Keep `Runner.running` true until the complete run finishes.
- Record download failures as `failed` or retry them.
