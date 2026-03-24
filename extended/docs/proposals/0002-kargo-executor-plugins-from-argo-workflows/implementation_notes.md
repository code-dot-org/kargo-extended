# 0002 Implementation Notes

- Host-side StepPlugin code lives under `extended/pkg/stepplugin/`.
- Argo-shaped copied or adapted helpers live under
  `extended/pkg/argoworkflows/`.
- The public CLI command is `kargo step-plugin build DIR`.
- The controller binary now includes `promotion-agent`.
- Plugin-backed Promotions use a per-Promotion agent pod named
  `promotion-agent-<promotion-uid>`.
- The agent pod main container runs builtin steps locally and plugin steps over
  localhost HTTP.
- The agent pod clones the current controller pod's main container env,
  envFrom, image, and mounted volumes, then adds:
  - shared workdir `emptyDir`
  - shared token `emptyDir`
  - explicit service account projected volume
- Plugin sidecars get only:
  - their declared container spec
  - `/workspace`
  - `/var/run/kargo`
  - optional service account projection
- Discovery currently resolves plugin `ConfigMap`s on demand through the
  controller client. It does not yet maintain a watched in-memory registry.
- The repo's existing CLI e2e harness is `pkg/cli/tests/e2e.sh`. It is already
  a large stateful shell runner, so StepPlugin smoke coverage should extend it,
  not duplicate it.
- Planned smoke-test shape:
  - keep fork-owned smoke logic in `extended/tests/e2e_stepplugins.sh`
  - keep the non-`extended/` edit to `pkg/cli/tests/e2e.sh` to one thin hook
  - reuse the existing shell helpers, login flow, and cluster assumptions
  - support `STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh` for the short path
    that runs shared bootstrap plus only the StepPlugin smoke phase
- The first StepPlugin smoke target is one real `mkdir` proof:
  - build the documented example with `kargo step-plugin build`
  - install the generated `ConfigMap`
  - run a `Stage` with `uses: mkdir`
  - use a later builtin step to prove the plugin-created directory exists in
    shared `/workspace`
- The agent pod does not keep live references to controller-namespace
  dependencies. Instead it mirrors only the referenced ConfigMaps and Secrets
  into the promotion namespace, rewrites `envFrom`, direct env key refs, and
  mounted volume sources to those mirrored names, and keeps those mirrored
  objects owned by the `Promotion`.
- The agent pod still does not carry over controller init containers.
- The init container keeps `/var/run/kargo` writable so it can write per-plugin
  bearer tokens before the main and sidecar containers start.
- The in-pod `promotion-agent` now uses uncached controller-runtime clients
  instead of `pkg/server/kubernetes.NewClient()`. The cached client path tried
  to list/watch cluster-scoped resources and blocked startup under the project
  namespace default service account.
- Post-green diff minimization was run against `upstream/main` after
  `git fetch upstream`.
- External code files reviewed in that pass:
  - `cmd/cli/root.go`
  - `cmd/controlplane/controller.go`
  - `cmd/controlplane/root.go`
  - `pkg/controller/promotions/promotions.go`
  - `charts/kargo/templates/controller/cluster-roles.yaml`
  - `pkg/cli/tests/e2e.sh`
- Result of the minimization pass:
  - the remaining external code edits are already thin seams or minimal hooks
  - no further safe helper extraction was found
  - the StepPlugin smoke path in `pkg/cli/tests/e2e.sh` now passes end to end
  - the full repo e2e script can still fail later for a separate project
    recreate race after delete
- Go review on the unstaged runtime fixes found two real regression risks:
  - dropping controller `envFrom` from the agent main/init containers
  - dropping non-`EmptyDir` mounted dependencies such as secret/configmap
    volumes needed by builtin steps
- The intended fix is not to keep inheriting controller namespace references
  directly. The intended fix is to mirror only referenced dependency objects
  into the promotion namespace, rewrite the pod spec to those mirrored names,
  and keep those mirrored objects owned by the `Promotion`.
- E2E runtime bugs found and fixed before that review:
  - duplicate service account mount paths in the agent pod
  - agent pod reusing the controller ServiceAccount name across namespaces
  - missing controller RBAC for pod create/get/delete
  - missing controller RBAC for mirrored ConfigMap and Secret create/update
  - init container trying to write to a read-only auth mount
  - in-pod `promotion-agent` blocking startup on cached cluster-scoped watches
- The smoke path now proves the real `mkdir` example end to end, but the full
  repo e2e script still has a later unrelated project recreate race after
  delete.
- The current full-repo e2e red is after the StepPlugin smoke path, in
  `pkg/cli/tests/e2e.sh` section 17:
  - around line 1402 the harness deletes the test project
  - around line 1405 it waits with a fixed `sleep 15`
  - around line 1418 it reapplies the same test project
- The observed failure at that point was:
  - `Error: apply resource: [PUT /v1beta1/resources] UpdateResource (status
    500): {}`
- The observed cluster state during that red was that the project namespace was
  still `Terminating` when the recreate happened.
- Most likely cause: the harness assumes namespace teardown will finish within
  15 seconds, but project deletion is asynchronous and can leave the namespace
  terminating past that fixed sleep.
- Most likely fix: replace the fixed sleep with a real wait loop that polls for
  full project or namespace disappearance before recreating the test project.
- This looks like a pre-existing repo harness race, not a StepPlugin-specific
  failure, because the new smoke path already passed before the later red.
- The full current `pkg/cli/tests/e2e.sh` was rerun after reproducing that
  exact red.
- The fix was a small `wait_for_project_deletion()` helper in
  `pkg/cli/tests/e2e.sh` plus replacing the fixed `sleep 15` with a real wait.
- The wait polls until both of these are true:
  - the Project namespace no longer exists
  - `kargo get projects <name>` no longer finds the Project
- After that fix, the full current repo harness passed:
  - `Tests Passed: 238`
  - `Tests Failed: 0`
- A fresh post-green diff minimization pass was rerun against `upstream/main`
  after the harness fix.
- Result of that follow-up minimization pass:
  - the extra `pkg/cli/tests/e2e.sh` helper and call site were small enough
    and clearer than trying to hide the wait in a stranger shell construct
  - no further safe shrink was found for the `pkg/cli/tests/e2e.sh` external
    seam
- A fresh Go review/fix loop after the harness fix found no new Go findings.
