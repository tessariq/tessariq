# Manual Test Report: TASK-032 Container Security Hardening

## Summary
All test cases PASS via real execution and inspection.

## Results

### TC-1: Container has --cap-drop=ALL
**PASS** — Created a real Docker container via `container.Process` (the production code path), then ran `docker inspect --format={{.HostConfig.CapDrop}}` on it.

```
CapDrop: [ALL]
```

### TC-2: Container has no-new-privileges
**PASS** — Same container inspected with `docker inspect --format={{.HostConfig.SecurityOpt}}`.

```
SecurityOpt: [no-new-privileges]
```

### TC-3: Evidence directory permissions
**PASS** — Called each production `Write*` function to create a real evidence directory, then checked with `os.Stat().Mode().Perm()`.

```
Evidence directory: 0700 PASS
```

### TC-4: Evidence file permissions
**PASS** — Created all evidence files through production code paths and verified each file's permissions on disk.

```
manifest.json        0600 PASS
status.json          0600 PASS
timeout.flag         0600 PASS
run.log              0600 PASS
runner.log           0600 PASS
agent.json           0600 PASS
runtime.json         0600 PASS
workspace.json       0600 PASS
```

### TC-5: Repair container is not hardened
**PASS** — `grep -n 'cap-drop\|security-opt\|no-new-privileges' internal/workspace/provision.go` returns no matches. The repair container in `repairWorkspaceOwnership()` uses a direct `docker run --rm --user root` invocation without any capability dropping or privilege escalation prevention, as required.

## Method
Manual tests executed via two throwaway Go programs (`cmd/manual-test-032/` and `cmd/manual-test-032-perms/`) that exercise the real production code paths and inspect actual Docker containers and filesystem state. These programs are ephemeral and deleted after testing per AGENTS.md policy.
