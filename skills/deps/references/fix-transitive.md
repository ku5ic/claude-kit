# /deps Phase 5: manual remediation of alerts with no PR (opt-in, best-effort)

Runs only when the arguments contain `--fix-transitive`. Without it, alerts with no Dependabot PR are reported in Phase 6 and left for the user; manual transitive remediation is ecosystem-specific and easy to get wrong unattended.

For each open alert with no PR, work in this order and stop at the first that applies:

1. Fixed version from `security_vulnerability.first_patched_version.identifier`. If null, there is no patched release; record "no fix available" and leave open.
2. Prefer the real fix: bump the parent. Using the injected manager's dependency-tree query, find the direct dependency that pulls the vulnerable transitive in. If a newer version of that direct dependency depends on a fixed version of the transitive, bump the direct dependency. This resolves the alert through normal resolution and leaves no standing pin.
3. Only if no parent bump resolves it (parent abandoned, or its latest still pulls the vulnerable range):
   1. Determine the transitive-pin mechanism from the injected manager itself.
   2. If you are not certain the ecosystem supports a transitive pin, stop and hand back to the user with the alert details rather than guessing.
   3. Apply the transitive-pin mechanism for the injected manager.
   4. Note: a transitive pin is debt - it persists even after the ecosystem moves on.
   5. Record every pin applied.
4. If neither applies and the advisory is not reachable in this project's usage, record it for dismissal rather than forcing a change.

After any manifest edit:

1. Confirm before writing.
2. Regenerate the lockfile with the manager's lockfile-only install.
3. Verify with the manager's tree query that the tree resolved to the fixed version.
4. Run `run-checks.sh`.
5. Branch.
6. Commit.
7. `gh pr create`.
8. Do not push to a protected branch; stop before merging your own PR.
