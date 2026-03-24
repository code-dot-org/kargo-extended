<kargo-extended>

Directions inside the main `<kargo-extended></kargo-extended>` block (which we
are in now) should supersede those in the rest of this file, as these are 
fork-specific overrides. The rest of the file is from upstream kargo, and is
very useful to follow, but kargo-extended fork directions have primacy if/when
there's any konflict.

# kargo-extended is a fork of kargo

This repo (kargo-extended) is a fork of Kargo: https://github.com/akuity/kargo.
The fork adds a minimal plugin / extension system, allowing "out of repo" features
to be added to Kargo.

## extended/ is our fork's primary directory

- `extended/` is the fork-owned surface area. Prefer putting new work there.
- Keep as much of code and files as possible under the `extended/` dir.
- Only change files outside `extended/` as minimally as possible to reduce
  merge conflicts with upstream.

## Merge Conflict Discipline Avoidance OUTSIDE `extended/`: keep it short by importing code from `extended/`

- PRIMARY GOAL DURING ALL IMPLEMENTATIONS: minimize both total changed lines and the number of
  non-contiguous edit blocks in files outside `extended/`.
- Agents should remember that re-implementation of complex systems carries a
  high risk of breakage not by merge commits, but by compatibility failing
  with upstream.
- Thus, the perfect edit minimizes lines, balanced by minimizing complexity
  that has to match between Kargo code and our code.
- This is a balance, not an absolute.
- Prefer the smallest reasonable design, not the smallest outside diff at any
  cost.
- Do not re-implement complex upstream subsystems under `extended/` just to
  avoid a small or stable upstream edit.
- Prefer thin bridge edits outside `extended/`.
- A thin bridge edit is a small edit in a file outside `extended/` whose job is
  only to wire, delegate, or inject, connecting to the real code in a helper
  file under `extended/`, usually named like `<upstream_basename>_bridge.go`.
  Shift as much code as possible out of the non-`extended/` file and into that
  helper file. This will look different from normal upstream code, and that is
  fine. We are not upstream Kargo. We are optimizing for avoiding merge
  conflicts.
- When touching a file outside `extended/`, do not optimize for making that
  file look like perfect upstream-owned code. Optimize for the fewest edits
  likely to conflict on future upstream rebases.
- Put real logic, helpers, adapters, and new types in libraries under
  `extended/` whenever Go package boundaries allow it.
- Load code from `extended/` helper libraries instead of defining new logic in
  files outside `extended/`.
- Expect each edited file outside `extended/` to have a corresponding helper or
  adapter in `extended/` when that reduces merge-conflict risk.
- For every feature seam that forces an edit outside `extended/`, add tests
  under `extended/` that verify the behavior behind that seam.
- Treat those `extended/` tests as the safety net for future merge-conflict
  repairs in files outside `extended/`.
- After making an external edit work, do an explicit follow-up pass:
  - ask how many of those edits can be removed by moving more logic behind an
    `extended/` helper
  - shrink the outside-`extended/` diff if that is practical
- After editing a file outside `extended/`, compare it against upstream Kargo
  and look for a better whole-file strategy that reduces total diff size and
  edit-block count.
- After resolving a merge conflict in a file outside `extended/`, run the
  corresponding `extended/` tests before trusting the merge.
- Treat this like code golf for external files: iterate on how little you need
  to change relative to upstream, but don't do anything too weird like boil the ocean.
- If an approach needs broad edits to a file outside `extended/`, stop and ask
  whether more of that logic can move under `extended/` first.

## Running E2E Tests

- If asked to run `pkg/cli/tests/e2e.sh`, first follow
  `docs/docs/60-contributor-guide/10-hacking-on-kargo.md` for local cluster
  and Tilt setup.
- Prefer a temporary `KUBECONFIG` in your shell if you need to use kind for
  e2e work and do not want to mutate the user's global kube context.
- On a fresh kind/Tilt cluster, `pkg/cli/tests/e2e.sh` currently assumes the
  singleton `ClusterConfig/cluster` already exists. If `kargo get
  clusterconfig` returns `404`, seed it before running e2e:

  ```bash
  kubectl get clusterconfig.kargo.akuity.io cluster >/dev/null 2>&1 || cat <<'EOF' | kubectl apply -f -
  apiVersion: kargo.akuity.io/v1alpha1
  kind: ClusterConfig
  metadata:
    name: cluster
  spec: {}
  EOF
  ```

- For our kargo-extended e2e tests only, use:
  `STEPPLUGINS_ONLY=true ./pkg/cli/tests/e2e.sh`
- To run their upstream kargo suite without our tests, use:
  `STEPPLUGINS_SKIP=true ./pkg/cli/tests/e2e.sh`
- To run both e2e suites (usually do this), run:
  `./pkg/cli/tests/e2e.sh`

## Technical Proposals

- Technical proposals live in `extended/docs/proposals/`.
- When researching technical work, look for relevant past proposals first, especially
  `trial`, `accepted` ones. You can grep proposals/ by status.yaml to find active proposals.
- When a proposal moves between phases, update its status in `proposal.md` and
  `status.yaml` if `status.yaml` exists.
- Major technical initiatives done by agents should automatically write a
  proposal under `extended/docs/proposals/`, see [section below](#implementing-new-features-and-the-proposal-process)
- Before you start oredit a proposal ALAWYS read `extended/docs/AGENTS.md` and
  `extended/docs/proposals/0000-proposal-directory-structure/proposal.md` FIRST
  for notes on writing, decision logging, and style.
- When changing a proposal's status, update both `proposal.md` and
  `status.yaml` if `status.yaml` exists.
- Sometimes we'll create two completing proposals and reject one after some iteration.
- @Codex: linking to proposals in chat (good when you refer to them!), make the text
  of the link [$proposal_dirname]($path_to_proposal_md), NOT [proposal.md]($path_to_proposal.md).
  I have to haver to see the path overwise and I can't tell which proposal it is.
  Example DO: [0000-proposal-directory-structure](extended/docs/proposals/00000-proposal-directory-structure/proposal.md)
- Keep in mind "what's the current proposal?" and "what are we working on?". We might make
  detours, and it can be very helpful to help me remember what we're up to if
  I ask.
- If we seem to be heading in a new direction, after some exploring, it can be
  very helpful to ask me questions like "Want to start a new proposal for this?"
- Remember, multiple proposals might be worked on in different chats/agents at once
  so keep track in your context any proposal (or proposals!) we are working on. Its
  Important to preserve these memories when compacting context!!!!

## Implementing new Features and the Proposal Process

1. For large changes ("new features"), agents should by default create a
   proposal when we start discussing it. Ask me if you're not sure if I want a
   proposal.
2. Update the proposal as we discuss tradeoffs conversationally and I make
   decisions.
3. When I tell you to implement, write `implementation_plan.md` first. Follow
   proposal `0000` guidelines in
   `extended/docs/proposals/0000-proposal-directory-structure/proposal.md`.
4. Derive `implementation_checklist.md` from `implementation_plan.md`. Write it
   in phases with checkable items.
5. When implementing a proposal, it is encouraged to use a branch named like
   `proposal/NNNN-proposal-dir-name`.
6. The PR title should match the current title in `proposal.md`, but with a
   valid Conventional Commits prefix. For new proposal implementation work,
   prefer `feat:`.
7. When implementation starts, update the PR description to include:
   - a GitHub link to the proposal directory on the implementation branch
   - a direct GitHub link to `proposal.md` on the implementation branch
   - the proposal text if useful once implementation is underway
8. If a PR is opened during proposal writing, do not churn its description
   until implementation starts.
9. If implementation is requested and no PR exists yet, ask whether you should
   create a PR and a proposal-named branch. Then suggest continuing in a fresh
   agent or fresh context on that branch.
10. Update both plan and checklist as you go when new information changes the
   shape of the work, especially `implementation_checklist.md`.
11. Add `implementation_notes.md` as you make decisions about things not in the
   proposal that a future agent might want to know, for example file paths,
   names, functions, architecture, and similar details.

## Recommended Phase To Add To All implementation_plans.md

If an implementation edits any file outside `extended/` (or you could imagine
it might by the time feature work is done), its
`implementation_plan.md` should probably include this at the end, and the
derived `implementation_checklist.md` should mirror it, with a phase named
exactly:

`Phase Post-Green: Minimize Diff Of Files Outside ./extended Against Kargo Upstream`

Use this procedure:

1. Get the feature green first.
   Start from a working tree where the relevant tests already pass. Do not do
   this phase first.
2. Use the real upstream Kargo history as the comparison base.
   Preferred remote is `upstream`. Preferred branch is `upstream/main`. If
   `upstream` does not exist, add the Akuity Kargo remote first.
3. Fetch before reviewing diffs.
   Run `git fetch upstream` so the comparison base is current enough.
4. Review every file outside `extended/` that was edited for this feature.
   For each one:
   - diff it against `upstream/main`
   - count changed lines
   - count non-contiguous edit blocks
5. Re-evaluate each outside-`extended/` edit now that the full feature works.
   Ask:
   - which edits were only convenient while building, but are no longer needed
   - which edits can move behind helpers or adapters under `extended/`
   - whether a different whole-file strategy collapses two blocks into one or
     removes a block entirely
6. If the file already had pre-existing fork drift against upstream, still do
   this pass.
   The goal is not only to minimize new edits. The goal is to see whether your
   work can ride the existing drift more cleanly or shrink the whole-file diff.
   Do not rewrite unrelated drift unless that really reduces the fork diff and
   stays safe.
7. Optimize for fewer merge-conflict surfaces, not prettier upstream-owned
   files.
   Prefer one small ugly bridge over a broad pretty rewrite.
8. Balance diff reduction against compatibility risk.
   Do not create large duplicate subsystems under `extended/` just to avoid a
   small upstream edit. If avoiding an upstream edit requires substantial
   parallel orchestration or behavior that must track upstream closely, the
   upstream edit may be the safer choice.
9. After every cleanup pass on an outside-`extended/` file:
   - rerun the matching tests under `extended/`
   - rerun any broader targeted tests affected by that seam
   - if the cleanup makes the diff worse or the code less obvious, back it out
10. End this phase only when:
   - no obvious helper extraction remains
   - no obvious edit block can be collapsed or removed
   - the matching `extended/` tests are green again


@CODEX: if you are reading this you must RIGHT NOW: find the current active
proposal, if any, especially if its being implemented (check
the implementation_checklist.md files). RIGHT NOW, read all associated files
with this proposal. Also, RIGHT NOW, please read in extended/docs/AGENTS.md
which contains tips on the type of writing we expect in chats and in MD files
you write. Think OpenBSD man pages or LKML posts, not corporate Oracle J2EE
architect longdoc blather. Clean, detailed, terse.

@CODEX: if you are implementing a proposal, and you notice its been a long
time since you last read the proposal docs (they are far back in your
context), RE-READ the proposal and all its docs.

@CODEX: if you are working on writing or editing or refining a proposal,
periodically re-read extended/docs/AGENTS.md to re-emphasize the writing
style we expect, and keep it near the top of your mind/context.

DO NOT EDIT BELOW THIS LINE OR WE WILL HAVE MERGE CONFLICTS WITH UPSTREAM KARGO

</kargo-extended>

# Kargo

Kargo is a Kubernetes-native continuous promotion platform for GitOps workflows.
Warehouses watch for new artifacts (container images, Git commits, Helm charts),
bundle them together as Freight, and promote that as a unit through a pipeline
of Stages.

Agent should reference `docs/docs/` if a more comprehensive understanding of the
domain is needed.

## Project Layout

| Path | Description |
|------|-------------|
| `api/v1alpha1/` | CRD types (Warehouse, Freight, Stage, Promotion, Project, etc.) and protobuf specs. Separate Go module |
| `cmd/controlplane/` | Back end entry point -- single binary with subcommands: `api`, `controller`, `management-controller`, `kubernetes-webhooks`, `external-webhooks`, `garbage-collector` |
| `cmd/cli/` | CLI entry point |
| `pkg/server/` | API server handlers. Two coexisting APIs: **ConnectRPC** (DEPRECATED, removal in v1.12.0; still used by UI -- avoid investing in fixes or enhancements) and **REST API** (the replacement; used by CLI; UI has not yet migrated) |
| `pkg/cli/` | CLI -- Cobra-based, subcommands in `pkg/cli/cmd/`, REST API client in `pkg/cli/client/` |
| `pkg/controller/` | Kubernetes controllers for Kargo resources |
| `pkg/promotion/` | Promotion engine and step runner |
| `pkg/promotion/runner/builtin/` | Built-in promotion steps (git, helm, kustomize, etc.) |
| `pkg/gitprovider/` | Git provider integrations (GitHub, GitLab, Gitea, BitBucket) |
| `pkg/image/` | Container image registry operations |
| `pkg/credentials/` | Secrets/credentials management |
| `pkg/webhook/` | Webhook handlers |
| `ui/` | React/TypeScript frontend (Vite + Ant Design + TanStack Query) |
| `charts/kargo/` | Helm chart -- primary installation method |

## Build & Development

### Prerequisites

Make, Go, Node.js, pnpm, and Docker are the primary prerequisites. Appropriate
versions of most other tools (golangci-lint, buf, controller-gen, swag, etc.)
are installed automatically in `hack/bin/` by Make targets.

### Common commands

```bash
make lint-go              # Lint Go code (golangci-lint)
make lint                 # Lint everything (Go, proto, charts, UI)
make format-go            # Auto-format Go code
make test-unit            # Run unit tests (with -race)
make build-cli            # Build CLI binary
make codegen              # Run all code generation
make hack-build-dev-tools # Build dev container with all tools
```

Containerized equivalents (no local tool installs needed):

```bash
make hack-lint-go
make hack-test-unit
make hack-codegen
```

Build the Kargo image:

```bash
make hack-build           # Build container image (kargo:dev)
```

There is seldom a need to do so directly.

### Local development with Tilt

```bash
make hack-kind-up          # Create local K8s cluster (or hack-k3d-up)
make hack-tilt-up          # Start local dev environment
```

`hack-kind-up` / `hack-k3d-up` are not needed if using OrbStack or Docker
Desktop's built-in Kubernetes clusters.

Tools like `tilt`, `ctlptl`, `kind`, `k3d`, and `helm` are auto-installed into
`hack/bin/` by these targets -- no manual installation needed. Tilt also handles
installing prerequisites (cert-manager, Argo CD, Argo Rollouts) idempotently.

- Tilt compiles back end on source changes
- **Manual trigger mode** used for re-deploying to the cluster -- trigger
  re-deployment from the Tilt UI (http://localhost:10350) or `hack/bin/tilt
  trigger <component>`
- API: localhost:30081, UI: localhost:30082, External webhooks: localhost:30083
- Argo CD: localhost:30080 (admin/admin)
- Kargo admin password: `admin`
- `make hack-tilt-down` to undeploy Kargo (preserves prerequisites)
- `make hack-kind-down` / `make hack-k3d-down` to destroy the cluster entirely

### Code generation

Run `make codegen` (or `make hack-codegen`) after modifying:

- API types in `api/v1alpha1/`
- Protobuf definitions
- Swagger annotations in `pkg/server/`
- JSON schemas for promotion step configs

This generates: protobuf bindings, CRDs, deepcopy methods, OpenAPI specs,
TypeScript API clients, and Go client code.

## Code Conventions

### Principles

- Clear over clever
- Simple over complex -- don't over-engineer or prematurely optimize
- Break large problems into small, well-defined pieces
- Structure code for testability
- Always include unit tests for new and modified code
- Never disable or skip a failing test -- fix the underlying problem

### Go style

- **Line length**: soft limit 80, hard limit 120. Don't sacrifice readability
  to hit 80 -- a few characters over is fine. Use `nolint: lll` when exceeding
  120 is truly unavoidable
- **Errors**: stdlib `errors` package only; never `github.com/pkg/errors`
- **Error handling**: always handle errors (except fmt.Print variants)
- **Naked returns**: forbidden
- **Exports**: unexported by default. Find a reason to export, not a reason
  not to
- **Package-level variables**: minimize; prefer passing dependencies explicitly,
  i.e. dependency injection
- **Variable shadowing**: forbidden (enforced by govet)
- **Import order** (enforced by gci): stdlib, third-party,
  `github.com/akuity`, dot, blank
- **var-naming**: linter rule disabled due to protobuf naming conflicts, but
  still follow Go conventions (`ID`, `URL`, `HTTP`) except where
  protobuf-generated code makes this impossible
- **Constants over literals**: extract repeated literals into named constants.
  In tests, inline literals are fine when extracting them hurts readability
- **Generated files**: `*_types.go` and `groupversion_info.go` are excluded
  from linting

### YAML style

Avoid the extra indent for list items:

```yaml
# Good
items:
- name: foo
  value: bar

# Avoid
items:
  - name: foo
    value: bar
```

### Readability

Write for a small viewport -- assume reviewers may read on a phone. These
guidelines can be bent when strict adherence hurts more than it helps.

**Single-field structs on one line:**

```go
// Good
foo := MyStruct{Name: "bar"}
items := []Item{{Name: "only-one"}}

// Avoid
foo := MyStruct{
    Name: "bar",
}
```

**One argument per line when breaking across lines.** Applies to definitions
and invocations:

```go
// Good
func NewServer(
    addr string,
    handler http.Handler,
    logger *slog.Logger,
) *Server {

// Avoid
func NewServer(addr string,
    handler http.Handler, logger *slog.Logger,
) *Server {
```

Related arguments may share a line (e.g. key/value pairs in structured logging):

```go
logger.Info(
    "promotion completed",
    "stage", stage.Name,
    "freight", freight.Name,
)
```

**Closing delimiters align with the opening statement:**

```go
// Good
results, err := client.Query(
    ctx,
    query,
    args,
)

// Avoid
results, err := client.Query(
    ctx,
    query,
    args)
```

**Group delimiters for single-element composites; separate for multiple:**

```go
// Single element -- grouped
items := []Item{{
    Name:  "only-one",
    Value: value,
}}

// Multiple elements -- separate
items := []Item{
    {
        Name:  "first",
        Value: val1,
    },
    {
        Name:  "second",
        Value: val2,
    },
}
```

### Testing

- **Framework**: testify (prefer `require` over `assert`)
- **Pattern**: table-driven tests with `t.Run()` subtests. Each case typically
  includes a `name` for identification and an `assert func(*testing.T, ...)`
  field for flexible outcome verification. White-box testing is fine -- order
  cases to exercise logical paths through the code under test from top to bottom
- **Parallelism**: `t.Parallel()` where possible
- **Mocking**: manual fake implementations via interfaces; no mock frameworks.
  Kubernetes tests use controller-runtime's `fake.NewClientBuilder().Build()`
- **Test location**: same package as the code under test

Example -- table-driven test with per-case assertions:

```go
func TestGetAuthorizedClient(t *testing.T) {
    testInternalClient := fake.NewClientBuilder().Build()
    testCases := []struct {
        name     string
        userInfo *user.Info
        assert   func(*testing.T, libClient.Client, error)
    }{
        {
            name: "no context-bound user.Info",
            assert: func(t *testing.T, _ libClient.Client, err error) {
                require.Error(t, err)
                require.Equal(t, "not allowed", err.Error())
            },
        },
        {
            name: "admin user",
            userInfo: &user.Info{IsAdmin: true},
            assert: func(t *testing.T, c libClient.Client, err error) {
                require.NoError(t, err)
                require.Same(t, testInternalClient, c)
            },
        },
    }
    for _, testCase := range testCases {
        t.Run(testCase.name, func(t *testing.T) {
            ctx := context.Background()
            if testCase.userInfo != nil {
                ctx = user.ContextWithInfo(ctx, *testCase.userInfo)
            }
            client, err := getAuthorizedClient(nil)(
                ctx, testInternalClient, "",
                schema.GroupVersionResource{}, "",
                libClient.ObjectKey{},
            )
            testCase.assert(t, client, err)
        })
    }
}
```

### Common Patterns

#### Constructors

Use an exported `New*` function returning an exported interface. Keep the
implementing struct unexported so callers depend on behavior, not
implementation:

```go
// credentials/database.go -- exported interface
type Database interface {
    Get(ctx context.Context, namespace string, credType Type, repo string) (*Credentials, error)
}

// credentials/kubernetes/database.go -- unexported implementation
type database struct {
    controlPlaneClient client.Client
    // ...
}

// credentials/kubernetes/database.go -- constructor returns interface
func NewDatabase(
    controlPlaneClient client.Client,
    localClusterClient client.Client,
    credentialProvidersRegistry credentials.ProviderRegistry,
    cfg DatabaseConfig,
) credentials.Database {
    return &database{
        controlPlaneClient:          controlPlaneClient,
        localClusterClient:          localClusterClient,
        credentialProvidersRegistry: credentialProvidersRegistry,
        cfg:                         cfg,
    }
}
```

#### Component registries

Self-registering component registries backed by `pkg/component`.
Implementations register in `init()`; the right one is selected at runtime by
name or predicate.

**Two flavors:**

- **Name-based** (`component.NameBasedRegistry`) -- O(1) lookup by string key
  (e.g. promotion steps)
- **Predicate-based** (`component.PredicateBasedRegistry`) -- sequential
  predicate evaluation until first match (e.g. webhook receivers, credential
  providers)

**Name-based example** -- promotion step runners:

```go
// pkg/promotion/runner/builtin/file_copier.go
func init() {
    promotion.DefaultStepRunnerRegistry.MustRegister(
        promotion.StepRunnerRegistration{
            Name:  stepKindCopy,
            Value: newFileCopier,
        },
    )
}
```

**Predicate-based example** -- webhook receivers:

```go
// pkg/webhook/external/github.go
func init() {
    defaultWebhookReceiverRegistry.MustRegister(
        webhookReceiverRegistration{
            Predicate: func(
                _ context.Context,
                cfg kargoapi.WebhookReceiverConfig,
            ) (bool, error) {
                return cfg.GitHub != nil, nil
            },
            Value: newGitHubWebhookReceiver,
        },
    )
}
```

When adding a new implementation: define it in its own file, self-register in
`init()`, and ensure the package is imported (usually via blank import at the
wiring location).

#### Environment-based configuration

Components define a companion config struct with `envconfig` tags and an
exported `*ConfigFromEnv()` function:

```go
// pkg/garbage/collector.go
type CollectorConfig struct {
    NumWorkers          int           `envconfig:"NUM_WORKERS" default:"3"`
    MaxRetainedFreight  int           `envconfig:"MAX_RETAINED_FREIGHT" default:"20"`
    MinFreightDeletionAge time.Duration `envconfig:"MIN_FREIGHT_DELETION_AGE" default:"336h"`
}

func CollectorConfigFromEnv() CollectorConfig {
    cfg := CollectorConfig{}
    envconfig.MustProcess("", &cfg)
    return cfg
}
```

#### Context propagation

Typed values stored in `context.Context` using unexported key types to prevent
collisions:

```go
// pkg/server/user/user.go
type userInfoKey struct{}

func ContextWithInfo(ctx context.Context, u Info) context.Context {
    return context.WithValue(ctx, userInfoKey{}, u)
}

func InfoFromContext(ctx context.Context) (Info, bool) {
    val := ctx.Value(userInfoKey{})
    if val == nil {
        return Info{}, false
    }
    u, ok := val.(Info)
    return u, ok
}
```

#### Error wrapping

Always wrap with `fmt.Errorf` and `%w`. Messages are lowercase (except when
starting with an exported type name), have no trailing punctuation, and the
wrapped error is always last. Two common phrasing styles:

```go
// "error <gerund>" -- most common
return fmt.Errorf("error listing projects: %w", err)
return fmt.Errorf("error getting runner for step kind %q: %w", req.Step.Kind, err)

// "failed to <infinitive>" -- also common
return fmt.Errorf("failed to create temporary directory: %w", err)
```

Either style is fine; be consistent within a file.

#### Logging

The `pkg/logging` package wraps `zap` with a simplified API. Loggers are stored
in context and enriched with key/value pairs as they flow through call chains:

```go
// Retrieve from context (falls back to global logger if absent)
logger := logging.LoggerFromContext(ctx)

// Enrich with request-scoped values, store back in context
logger = logger.WithValues("namespace", ns, "name", name)
ctx = logging.ContextWithLogger(ctx, logger)

// Log at various levels
logger.Trace("discovered commit", "tag", tag.Tag)   // very verbose
logger.Debug("routing webhook request")              // debugging detail
logger.Info("promotion completed", "stage", s.Name)  // normal operations
logger.Error(err, "error refreshing object")         // error takes err first
```

Levels (configured via `LOG_LEVEL` env var): `trace`, `debug`, `info`, `error`,
`discard`. Prefer `Debug` for operational detail, `Info` for state transitions,
`Error` for actionable failures. Use `Trace` sparingly for high-volume
discovery loops.

#### REST API structure

The REST API (`pkg/server/`) uses Gin. Routes are defined in
`rest_router.go` under `/v1beta1`. Gin middleware handles authentication,
error formatting, and request body limits.

Project-scoped routes live under `/v1beta1/projects/:project`. Middleware on
this group confirms the project exists before any handler runs, so individual
endpoints do not need to check this.

#### Authorization model

Kargo resources are Kubernetes-native, so authorization is largely implicit.
The API server uses an **authorizing client** (`pkg/server/kubernetes/client.go`)
that wraps the controller-runtime client and performs a `SubjectAccessReview`
before every operation. If the user lacks permission, the request fails before
any data is read or written.

**Custom verbs ("dolphin verbs"):** Kargo defines a `"promote"` verb for Stages.
Because this is not a standard Kubernetes CRUD verb, the authorizing client
cannot check it implicitly. Endpoints that require promote permission
(promote-to-stage, promote-downstream, approve-freight) must test for permission
using the wrapper's `Authorize()` method explicitly.

**Internal client bypass:** In rare cases the API server uses its own
(non-authorizing) internal client to act on behalf of a user. **Any code that
bypasses the authorizing client must be treated as security-sensitive:**
document the justification, test thoroughly, and ensure no path allows
unauthorized data access or mutation.

#### Pluggable methods (legacy -- do not introduce)

Many existing components use function-typed fields on structs to make behaviors
swappable for testing. The constructor wires each field to a real method, and
tests override specific fields:

```go
// pkg/garbage/collector.go
type collector struct {
    cfg              CollectorConfig
    cleanProjectFn   func(ctx context.Context, project string) error
    listProjectsFn   func(context.Context, client.ObjectList, ...client.ListOption) error
    deleteFreightFn  func(context.Context, client.Object, ...client.DeleteOption) error
    // ...
}

func NewCollector(kubeClient client.Client, cfg CollectorConfig) Collector {
    c := &collector{cfg: cfg}
    c.cleanProjectFn = c.cleanProject
    c.listProjectsFn = kubeClient.List
    c.deleteFreightFn = kubeClient.Delete
    return c
}
```

**This pattern is being phased out.** It is fine to continue using it (even
adding new `Fn` fields) where it already exists, but new components should
prefer interfaces and fake implementations. In most cases, controller-runtime's
`fake.NewClientBuilder().Build()` is sufficient for mocking Kubernetes
interactions.

## Documentation

The docs site lives in `docs/` and uses Docusaurus 3. Content is in
`docs/docs/` under numbered directories that control sidebar ordering. The doc
tree is organized by audience:

- **Quickstart** -- for anyone evaluating Kargo
- **Operator guide** -- for platform engineers installing and configuring Kargo
- **User guide** -- for end users promoting freight through stages
- **Contributor guide** -- for developers working on Kargo itself
- **Release notes** -- per-version changelogs

### Conventions

- **File naming**: `NN-kebab-case-name.md` -- numeric prefix controls sidebar
  order
- **Frontmatter**: use `sidebar_label` and `description` fields
- **Directory metadata**: `_category_.json` for folder labels, collapsibility,
  and generated index pages
- **Admonitions**: Docusaurus syntax (`:::note`, `:::info`, `:::caution`,
  `:::warning`). Favor `note` for things the reader should pay attention to.
  Favor `info` for supplemental information safe to skip. Use `warning` or
  `caution` sparingly -- reserve for common mistakes or insecure configurations
- **Tabs**: `<Tabs>` / `<TabItem>` for instructions that vary by OS, interface
  (dashboard vs CLI vs API), or other mutually exclusive options
- **UI elements**: use the `<hlt>` tag (e.g. `<hlt>Save</hlt>`) only for text
  that actually appears on screen -- not for abstract descriptions
- **"Freight" is a mass noun** (like "luggage"). Never pluralize as "freights"
  or use "a freight." Say "`Freight` resource" for the Kubernetes object or
  "piece of freight" for the abstract concept. Generated code may sometimes
  force incorrect pluralization -- accept it there rather than fighting the
  generator
- **Avoid "deploy"/"deployment"** when describing what Kargo does. Deploying is
  the job of a GitOps agent like Argo CD. Kargo *promotes*. Use "promote,"
  "promotion," or "progress"
- **Media assets**: keep close to the pages that reference them (same directory
  or sibling `img/` directory)
- **Refer to `docs/STYLE_GUIDE.md`** for phraseology, capitalization, and
  additional formatting conventions

### Building and previewing

```bash
make serve-docs          # Dev server on localhost:3000 (or $DOCS_PORT)
make hack-serve-docs     # Containerized version (no local Node.js needed)
```

Under the hood these run `pnpm install`, build a custom gtag plugin, then
start `docusaurus start`. To build the static site without serving:

```bash
cd docs && pnpm install && pnpm build-gtag-plugin && docusaurus build
```

## Workflow

### Definition of done

A task is complete when:

1. Code changes are implemented
2. Unit tests cover the changes
3. Linting passes (`make lint-go`, or `make lint` if non-Go files changed)
4. The project builds successfully
5. Documentation is updated if user-facing behavior changed

### Commits

All commits must include a DCO sign-off:

```plaintext
Signed-off-by: Legal Name <email@example.com>
```

Use `git commit -s` to add this automatically.

### Problem-solving

- **Read before guessing.** Understand existing code, tests, and error messages
  before proposing changes. Find similar features in the codebase and mirror
  their patterns. Grep for usage, read neighboring files, check tests
- **Stay focused.** Do what was asked -- no more, no less. If a tangential
  improvement seems valuable, mention it but don't act without approval.
  Exception: in docs already being modified for an in-scope reason, fixing
  obvious typos or markdown lint issues is welcome
- **Ask when ambiguous.** If requirements are unclear or there are multiple
  reasonable approaches, ask rather than guess. When choosing between valid
  approaches, prioritize: testability, readability, consistency with existing
  patterns, simplicity, reversibility
- **Three strikes, then ask.** If an approach fails three times, stop and
  explain what you've tried instead of continuing to iterate
- **Fix root causes.** Investigate why something fails rather than papering over
  symptoms. Don't disable linters, skip tests, or add `//nolint` without
  understanding the underlying issue
- **Minimize blast radius.** Prefer small, focused changes. If a fix touches
  many files, consider whether a simpler approach exists
