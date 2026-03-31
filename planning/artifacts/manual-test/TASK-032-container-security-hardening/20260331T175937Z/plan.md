# Manual Test Plan: TASK-032 Container Security Hardening

## Objective
Verify container security posture via docker inspect and evidence file permissions on disk.

## Test Cases

### TC-1: Container has --cap-drop=ALL
- Run a tessariq container that sleeps long enough to inspect
- Run `docker inspect --format='{{.HostConfig.CapDrop}}' <container>`
- Expected: `[ALL]`

### TC-2: Container has no-new-privileges
- Run `docker inspect --format='{{.HostConfig.SecurityOpt}}' <container>`
- Expected: `[no-new-privileges]`

### TC-3: Evidence directory permissions
- After a completed run, check evidence directory permissions
- Run `stat -c '%a' <evidence_dir>`
- Expected: `700`

### TC-4: Evidence file permissions
- Check each evidence file: manifest.json, status.json, agent.json, runtime.json, run.log, runner.log, task.md
- Run `stat -c '%a' <file>` for each
- Expected: `600`

### TC-5: Repair container is not hardened
- Inspect workspace/provision.go code path
- Verify repair container uses `docker run` directly without --cap-drop or --security-opt
- Expected: repair container runs as root with full capabilities

## Execution Method
Tests TC-1 through TC-4 are verified via integration and e2e tests that run real Docker containers. TC-5 is verified via code inspection.
