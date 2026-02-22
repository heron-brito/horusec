# Horusec - Claude Code Context

## Git Workflow Rules
- **Always work on a branch** prefixed with `fix-` (e.g., `fix-goreleaser-v2`, `fix-docker-auth`)
- **Never commit directly to `main`** — all changes must go through a Pull Request
- Stage only the changed files, commit, then create a PR from the current branch to `main` using `gh pr create`

---

## 1. Architecture Overview

Horusec is an open-source **security scanning CLI orchestrator** written in Go. It detects vulnerabilities across 18+ programming languages by running 22 security tools inside Docker containers and unifying their outputs.

### High-Level Flow
```
CLI Input (cobra flags/env/config)
    |
    v
Config Validation (usecases/cli)
    |
    v
Requirement Checks (Docker daemon, Git)
    |
    v
Analyzer Controller (orchestrator)
    |--- Language Detection (go-enry, file scanning)
    |--- Runner (per-language tool dispatch)
    |       |--- Formatter (per-tool adapter)
    |       |       |--- Docker API (container lifecycle)
    |       |       |--- Parse tool output -> Vulnerability
    |       |       '--- Commit Author enrichment (git blame)
    |       '--- Collect results into Analysis
    |--- Horusec API Client (optional, send to manager)
    '--- Print Results (text / JSON / SARIF / SonarQube)
```

### Layers (Clean Architecture)
| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| **Entry Point** | `cmd/app/` | Cobra CLI commands (start, generate, version) |
| **Configuration** | `config/` | Viper-based config, env vars, flags, file loading |
| **Controllers** | `internal/controllers/` | Orchestration: analyzer, language_detect, printresults, requirements |
| **Use Cases** | `internal/usecases/` | Business rules: config validation |
| **Services** | `internal/services/` | Docker API, formatters, git, horusec_api, custom_rules, sarif, sonarqube |
| **Entities** | `internal/entities/` | Domain models: docker, toolsconfig, workdir, custom_images |
| **Enums** | `internal/enums/` | Constants: images, outputtype, toignore |
| **Helpers** | `internal/helpers/` | Centralized error/info messages |
| **Utils** | `internal/utils/` | File ops, JSON, prompts, vuln_hash, copy |

---

## 2. Tech Stack

| Component | Technology | Version/Details |
|-----------|-----------|----------------|
| **Language** | Go | 1.22 (go.mod) |
| **Module** | `github.com/ZupIT/horusec/` | Main module path |
| **CLI Framework** | Cobra + Viper | Command parsing, flags, config file |
| **Docker SDK** | `docker/docker` | v26.1.5+incompatible |
| **Testing** | testify, ginkgo/gomega | Unit + BDD E2E |
| **Linting** | golangci-lint | @latest (v1.64.8+), config: `.golangci.yml` |
| **Build** | GoReleaser v2 | `.goreleaser.yaml`, `goreleaser-action@v6` |
| **CI/CD** | GitHub Actions | 15+ workflows |
| **Container Registry** | GHCR | `ghcr.io/heron-brito/horusec-*` |
| **Language Detection** | go-enry | Detects languages from file extensions/content |
| **Validation** | ozzo-validation | Fluent config validation |
| **Logging** | logrus | Structured logging with levels |
| **HTTP Client** | net/http | Horusec Manager API communication |
| **Output Formats** | text, JSON, SARIF v2.1.0, SonarQube | Multiple report formats |

---

## 3. Environment Variables

### HORUSEC_CLI_* (Main Configuration)

All are prefixed with `HORUSEC_CLI_` and defined in `config/config.go`:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `HORUSEC_CLI_HORUSEC_API_URI` | string | `http://0.0.0.0:8000` | Horusec Manager server address |
| `HORUSEC_CLI_TIMEOUT_IN_SECONDS_REQUEST` | int64 | 300 | HTTP request timeout (min 10s) |
| `HORUSEC_CLI_TIMEOUT_IN_SECONDS_ANALYSIS` | int64 | 600 | Analysis timeout (min 10s) |
| `HORUSEC_CLI_MONITOR_RETRY_IN_SECONDS` | int64 | 15 | Monitor retry (deprecated) |
| `HORUSEC_CLI_REPOSITORY_AUTHORIZATION` | string | `00000000-...` (nil UUID) | Horusec Manager auth token |
| `HORUSEC_CLI_PRINT_OUTPUT_TYPE` | string | `""` | Output: text, json, sarif, sonarqube |
| `HORUSEC_CLI_JSON_OUTPUT_FILEPATH` | string | `""` | File path for report output |
| `HORUSEC_CLI_SEVERITIES_TO_IGNORE` | []string | `["INFO"]` | Severities to skip |
| `HORUSEC_CLI_FILES_OR_PATHS_TO_IGNORE` | []string | `["*tmp*","**/.vscode/**"]` | Ignore patterns |
| `HORUSEC_CLI_RETURN_ERROR_IF_FOUND_VULNERABILITY` | bool | false | Exit 1 if vulns found |
| `HORUSEC_CLI_PROJECT_PATH` | string | cwd | Project path to analyze |
| `HORUSEC_CLI_WORK_DIR` | json | default WorkDir | Language-specific subdirectories |
| `HORUSEC_CLI_ENABLE_GIT_HISTORY_ANALYSIS` | bool | false | Enable Gitleaks git history scan |
| `HORUSEC_CLI_CERT_INSECURE_SKIP_VERIFY` | bool | false | Skip TLS verification |
| `HORUSEC_CLI_CERT_PATH` | string | `""` | CA certificate path |
| `HORUSEC_CLI_ENABLE_COMMIT_AUTHOR` | bool | false | Include git blame info |
| `HORUSEC_CLI_REPOSITORY_NAME` | string | dirname | Repo name for Horusec Manager |
| `HORUSEC_CLI_FALSE_POSITIVE_HASHES` | []string | `[]` | Hashes to mark false positive |
| `HORUSEC_CLI_RISK_ACCEPT_HASHES` | []string | `[]` | Hashes to mark risk accepted |
| `HORUSEC_CLI_TOOLS_CONFIG` | json | default ToolsConfig | Per-tool enable/disable config |
| `HORUSEC_CLI_HEADERS` | json | `{}` | Custom headers for API requests |
| `HORUSEC_CLI_CONTAINER_BIND_PROJECT_PATH` | string | `""` | Host path for Docker binding |
| `HORUSEC_CLI_DISABLE_DOCKER` | bool | false | Run without Docker containers |
| `HORUSEC_CLI_CUSTOM_RULES_PATH` | string | `""` | Custom rules JSON file path |
| `HORUSEC_CLI_ENABLE_INFORMATION_SEVERITY` | bool | false | Show INFO severity vulns |
| `HORUSEC_CLI_CUSTOM_IMAGES` | json | default CustomImages | Custom Docker images per language |
| `HORUSEC_CLI_SHOW_VULNERABILITIES_TYPES` | []string | `["Vulnerability"]` | Types to display |
| `HORUSEC_CLI_LOG_FILE_PATH` | string | `/tmp/horusec-{ts}.log` | Log file path |
| `HORUSEC_CLI_ENABLE_OWASP_DEPENDENCY_CHECK` | bool | false | Enable OWASP Dep Check |
| `HORUSEC_CLI_ENABLE_SHELLCHECK` | bool | false | Enable ShellCheck |

### Docker Registry Variables (`internal/services/docker/docker_api.go`)

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `HORUSEC_CLI_REGISTRY_USERNAME` | string | `""` | Docker registry username |
| `HORUSEC_CLI_REGISTRY_PASSWORD` | string | `""` | Docker registry password |
| `HORUSEC_CLI_REGISTRY_ADDRESS` | string | `""` | Docker registry server address |

### Tool-Specific

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub API token (required by Nancy to avoid rate limiting) |

### Config Load Order (later overrides earlier)
1. Hardcoded defaults
2. Config file (`horusec-config.json` or custom path)
3. Environment variables
4. CLI flags

---

## 4. Directory Structure

```
horusec/
|-- cmd/app/                          # CLI entry point
|   |-- main.go                       # Root Cobra command + global flags
|   |-- start/start.go                # "horusec start" command (30+ flags)
|   |-- generate/generate.go          # "horusec generate" config file generator
|   '-- version/version.go            # "horusec version" command
|
|-- config/                           # Configuration management
|   |-- config.go                     # GlobalOptions struct, viper, env binding
|   '-- dist/                         # Build-time distribution flags
|
|-- internal/
|   |-- controllers/
|   |   |-- analyzer/                 # Main analysis orchestrator + runner
|   |   |-- language_detect/          # Project language detection (go-enry)
|   |   |-- printresults/             # Output formatting (text/JSON/SARIF/SQ)
|   |   '-- requirements/             # Docker + Git requirement validation
|   |       |-- docker/
|   |       '-- git/
|   |
|   |-- entities/
|   |   |-- docker/                   # AnalysisData (image, cmd, language)
|   |   |-- toolsconfig/              # 22 tool configs (enable/disable)
|   |   |-- workdir/                  # Per-language subdirectory mapping
|   |   '-- custom_images/            # Custom Docker images per language
|   |
|   |-- services/
|   |   |-- docker/                   # Docker API client + container lifecycle
|   |   |   '-- client/               # Docker daemon connection config
|   |   |-- formatters/               # 50+ tool output parsers (by language)
|   |   |   |-- service.go            # Base formatter service (IService)
|   |   |   |-- interface.go          # IFormatter interface
|   |   |   |-- default_engine_formatter.go
|   |   |   |-- c/                    # Flawfinder
|   |   |   |-- csharp/               # SCS, DotnetCli, HorusecCSharp
|   |   |   |-- dart/                 # HorusecDart
|   |   |   |-- elixir/               # Sobelow, MixAudit
|   |   |   |-- generic/              # Trivy, Semgrep, OWASP DepCheck
|   |   |   |-- go/                   # GoSec, Nancy
|   |   |   |-- hcl/                  # TfSec, Checkov
|   |   |   |-- java/                 # HorusecJava
|   |   |   |-- javascript/           # HorusecJS, NpmAudit, YarnAudit
|   |   |   |-- kotlin/               # HorusecKotlin
|   |   |   |-- leaks/                # Gitleaks, HorusecLeaks
|   |   |   |-- nginx/                # HorusecNginx
|   |   |   |-- php/                  # PhpCS
|   |   |   |-- python/               # Bandit, Safety
|   |   |   |-- ruby/                 # Brakeman, BundlerAudit
|   |   |   |-- shell/                # ShellCheck
|   |   |   '-- yaml/                 # HorusecKubernetes
|   |   |-- engines/                  # Built-in regex rule engines
|   |   |   |-- csharp/               # C# rules
|   |   |   |-- dart/                 # Dart rules
|   |   |   |-- javascript/           # JS rules
|   |   |   |-- jvm/                  # Java/Kotlin rules
|   |   |   |-- kubernetes/           # K8s/YAML rules
|   |   |   |-- leaks/                # Secret detection rules
|   |   |   |-- nginx/                # Nginx rules
|   |   |   '-- swift/                # Swift rules (not in formatters yet)
|   |   |-- git/                      # Git blame / commit author extraction
|   |   |-- horusec_api/              # Horusec Manager API client
|   |   |-- custom_rules/             # User-defined custom rules loader
|   |   |-- sarif/                    # SARIF v2.1.0 report generator
|   |   '-- sonarqube/                # SonarQube report generator
|   |
|   |-- usecases/cli/                 # Config validation business rules
|   |-- enums/                        # images, outputtype, toignore
|   |-- helpers/messages/              # Centralized error/info messages
|   '-- utils/                        # file, json, prompt, copy, vuln_hash
|
|-- e2e/                              # End-to-end tests (ginkgo/gomega)
|   |-- commands/                     # CLI command E2E tests
|   '-- analysis/                     # Analysis pipeline E2E tests
|
|-- deployments/scripts/              # Build scripts, install.sh
|-- examples/                         # Example vulnerable projects (Go, etc.)
|-- assets/                           # Static resources
'-- .github/workflows/                # 15+ CI/CD workflows
```

---

## 5. Services, Controllers & Entities

### Controllers (4)

| Controller | File | Purpose |
|------------|------|---------|
| **Analyzer** | `controllers/analyzer/analyzer.go` | Main pipeline orchestrator: detect languages -> run tools -> send results -> print |
| **Runner** | `controllers/analyzer/runner.go` | Dispatches formatters per language via `detectVulnerabilityFuncs()` map |
| **LanguageDetect** | `controllers/language_detect/` | Scans project for programming languages using go-enry |
| **PrintResults** | `controllers/printresults/` | Outputs analysis in text/JSON/SARIF/SonarQube format |
| **Requirements** | `controllers/requirements/` | Validates Docker daemon + Git availability |

### Services (8)

| Service | Directory | Purpose |
|---------|-----------|---------|
| **Docker API** | `services/docker/` | Container lifecycle: create, start, wait, logs, remove |
| **Formatters** | `services/formatters/` | 50+ tool-specific output parsers (IFormatter interface) |
| **Engines** | `services/engines/` | Built-in regex rule engines for 8 languages |
| **Git** | `services/git/` | Git blame, commit author extraction |
| **Horusec API** | `services/horusec_api/` | HTTP client for Horusec Manager server |
| **Custom Rules** | `services/custom_rules/` | Load/validate user-defined security rules from JSON |
| **SARIF** | `services/sarif/` | SARIF v2.1.0 report generation |
| **SonarQube** | `services/sonarqube/` | SonarQube report generation |

### Entities (4)

| Entity | Package | Key Struct | Purpose |
|--------|---------|-----------|---------|
| **AnalysisData** | `entities/docker` | `AnalysisData` | Docker image, CMD, language for container execution |
| **ToolsConfig** | `entities/toolsconfig` | `ToolsConfig` | Map of 22 tools with enable/disable flags |
| **WorkDir** | `entities/workdir` | `WorkDir` | Per-language subdirectory paths (18 languages) |
| **CustomImages** | `entities/custom_images` | `CustomImages` | Map of language to custom Docker image |

### CLI Commands (3)

| Command | File | Description |
|---------|------|-------------|
| `horusec start` | `cmd/app/start/start.go` | Execute security analysis (30+ flags) |
| `horusec generate` | `cmd/app/generate/generate.go` | Generate `horusec-config.json` config file |
| `horusec version` | `cmd/app/version/version.go` | Display version, commit, build date |

---

## 6. Security Tools (22 total)

Defined in `internal/entities/toolsconfig/tools_config.go`. All enabled by default.

### SAST (9 tools)
| Tool | Language | Docker Image |
|------|----------|-------------|
| Bandit | Python | horusec-python |
| Brakeman | Ruby (Rails) | horusec-ruby |
| Flawfinder | C/C++ | horusec-c |
| GoSec | Go | horusec-go |
| PhpCS | PHP | horusec-php |
| SecurityCodeScan | C#/.NET | horusec-csharp |
| Semgrep | Multi-language | horusec-generic |
| Sobelow | Elixir/Phoenix | horusec-elixir |
| HorusecEngine | Multi-language | (built-in, no container) |

### Dependency Scanners (8 tools)
| Tool | Language | Docker Image |
|------|----------|-------------|
| BundlerAudit | Ruby gems | horusec-ruby |
| MixAudit | Elixir | horusec-elixir |
| NpmAudit | Node.js (npm) | horusec-js |
| YarnAudit | Node.js (yarn) | horusec-js |
| Safety | Python (pip) | horusec-python |
| OwaspDependencyCheck | Multi-language | horusec-generic |
| DotnetCli | C#/.NET (NuGet) | horusec-csharp |
| Nancy | Go modules | horusec-go |

### Secret Detection (1 tool)
| Tool | Scope | Docker Image |
|------|-------|-------------|
| GitLeaks | All languages / git history | horusec-leaks |

### Infrastructure-as-Code (2 tools)
| Tool | Target | Docker Image |
|------|--------|-------------|
| Checkov | Terraform, CloudFormation, K8s | horusec-hcl |
| TfSec | Terraform | horusec-hcl |

### Script Analysis (2 tools)
| Tool | Language | Docker Image |
|------|----------|-------------|
| ShellCheck | Shell/Bash | horusec-shell |
| Trivy | Containers/OS packages | horusec-generic |

---

## 7. Build & Dev Commands

```bash
make lint                    # golangci-lint (config: .golangci.yml)
make test                    # Unit tests: go test -race -timeout=5m -parallel=1 -failfast -short
make test-e2e                # E2E tests: builds all tools, 30min timeout
make coverage                # Coverage report (cmd/config=90%, internal=86%)
make build-dev               # Development build
make pipeline                # Full CI: format, lint, test, coverage, security
make format                  # goimports + gofumpt
```

### Go Version Note
- System default may be Go 1.18; project requires **Go 1.22**
- Build/test: `GOROOT=/usr/lib/go-1.22 GOTOOLCHAIN=local /usr/lib/go-1.22/bin/go test ./...`

---

## 8. Common Hurdles & Solutions (12)

### 1. GoReleaser v2 config format
**Problem:** `only version: 2 configuration files are supported, yours is version: 0`
**Solution:** Add `version: 2` header to `.goreleaser.yaml`. GoReleaser v2 (`goreleaser-action@v6` with `version: '~> v2'`) requires this field.

### 2. Docker Hub -> GHCR migration
**Problem:** `unauthorized: access token has insufficient scopes` when pushing images to Docker Hub.
**Solution:** Replace all `horuszup/horusec-*` image refs with `ghcr.io/heron-brito/horusec-*`. Use `docker/login-action@v3` with `registry: ghcr.io` and `GITHUB_TOKEN`.

### 3. Cosign v1.2.0 deprecated
**Problem:** `sigstore/cosign-installer` no longer supports `cosign-release: 'v1.2.0'`.
**Solution:** Update to `sigstore/cosign-installer@v3` or comment out cosign steps if COSIGN_KEY secret is not configured.

### 4. Go version mismatch (1.17 vs 1.22)
**Problem:** `package cmp/slices/net/netip is not in GOROOT` — Docker SDK v26 requires Go 1.21+ packages.
**Solution:** Update `go-version` in all workflow files from `1.17` to `1.22`. The `go.mod` declares `go 1.22`.

### 5. macOS ARM64 runners (no Docker)
**Problem:** `macOS-latest` resolves to ARM64 runners. Docker cannot run (no nested virtualization). `macos-13` was retired Dec 2025.
**Solution:** Remove macOS from test matrix or use `macos-15-large` (paid Intel runner). Docker-dependent tests only run on ubuntu/windows.

### 6. depguard v2 config format change
**Problem:** golangci-lint v1.64.8 ships `depguard/v2` which changed config format. Old `list-type: blacklist` creates a "Main" deny-list that blocks ALL imports (179 false positives).
**Solution:** Remove `depguard` from enabled linters in `.golangci.yml` (the original config had empty package lists — it was a no-op).

### 7. gci local-prefixes deprecated
**Problem:** `gci` linter reports "File is not properly formatted" on correct files. The `local-prefixes` config option is deprecated in newer gci versions.
**Solution:** Remove `gci` from enabled linters. `goimports` (already enabled) handles import ordering.

### 8. nolintlint leading space
**Problem:** `directive '// nolint:xxx' should be written without leading space as '//nolint:xxx'`
**Solution:** Replace all `// nolint` with `//nolint` (no space after `//`). Set `nolintlint.allow-leading-space: false` in `.golangci.yml`.

### 9. Deprecated linters in golangci-lint v1.64+
**Problem:** `deadcode`, `exportloopref`, `gomnd`, `structcheck`, `varcheck` are "fully inactivated".
**Solution:** Remove them from the `enable:` list in `.golangci.yml`. Remove any `//nolint:gomnd` directives.

### 10. Docker SDK v26 type renames
**Problem:** Compile errors after upgrading `docker/docker` to v26.1.5.
**Solution:** Key renames: `ContainerCreateCreatedBody` -> `CreateResponse`, `ContainerWaitOKBody` -> `WaitResponse`, `ContainerWaitOKBodyError` -> `WaitExitError`, `AuthConfig` moved to `types/registry`. Option types moved from `api/types` to `api/types/container`.

### 11. Safety v2 vs v3 breaking CLI
**Problem:** Safety v3 changed CLI syntax (`safety check --json` no longer works).
**Solution:** Pin `safety==2.3.5` in the Python Dockerfile. The Horusec formatter uses `safety check -r ... --json` syntax which requires v2.x.

### 12. golangci-lint @latest breaks builds
**Problem:** `make lint` runs `go install golangci-lint@latest` which can introduce breaking changes from new linter versions.
**Solution:** Consider pinning golangci-lint to a specific version in the Makefile. When upgrading, check for deprecated linters and config format changes.

---

## 9. Design Patterns (14)

### 1. Clean Architecture
**Where:** Entire project structure (`cmd/` -> `controllers/` -> `usecases/` -> `services/` -> `entities/`)
**How:** Clear separation of layers with dependency rule: outer layers depend on inner layers via interfaces.

### 2. Dependency Injection (Constructor-based)
**Where:** All `New()` constructors — `analyzer.New()`, `NewFormatterService()`, `NewStartCommand()`
**How:** Components receive dependencies through constructor parameters, returning interfaces. No global singletons.

### 3. Strategy Pattern
**Where:** `services/formatters/interface.go` (IFormatter), 50+ formatter implementations
**How:** Each formatter implements `StartAnalysis()` differently per tool. The runner selects the right strategy per language.

### 4. Factory Pattern
**Where:** `config.New()`, `analyzer.New()`, `runner.detectVulnerabilityFuncs()`
**How:** Factory functions create complex objects with proper initialization and dependency wiring.

### 5. Template Method Pattern
**Where:** `services/formatters/default_engine_formatter.go`, all formatter `StartAnalysis()` methods
**How:** Skeleton: check if tool is ignored -> execute container -> parse output -> add to analysis. Each formatter overrides parse step.

### 6. Adapter Pattern
**Where:** `services/docker/docker_api.go`, `services/formatters/service.go`
**How:** Adapts third-party APIs (Docker SDK, Git CLI) to application interfaces (`Docker`, `IService`).

### 7. Facade Pattern
**Where:** `controllers/analyzer/analyzer.go`, `services/formatters/service.go`
**How:** `Analyzer.Analyze()` provides a single entry point to the complex analysis pipeline. `Service` simplifies Docker + Git + output for formatters.

### 8. Builder Pattern
**Where:** `config/config.go` — `LoadGlobalFlags()`, `LoadStartFlags()`, `LoadFromConfigFile()`, `Normalize()`
**How:** Config is built step-by-step through chainable methods that each return `*Config`.

### 9. Command Pattern
**Where:** `cmd/app/main.go`, Cobra commands (`start`, `generate`, `version`)
**How:** Each CLI command encapsulates a request as an object with `RunE` handler.

### 10. Chain of Responsibility
**Where:** `controllers/analyzer/runner.go` — `detectVulnerabilityFuncs()` map, sequential formatter execution
**How:** Multiple formatters process the analysis in sequence. Each adds vulnerabilities independently.

### 11. Observer Pattern
**Where:** `controllers/printresults/` — `SetAnalysis()` + `Print()`
**How:** PrintResults controller observes the analysis object and renders output when analysis completes.

### 12. Decorator Pattern
**Where:** `services/formatters/service.go` — `SetCommitAuthor()`, `controllers/analyzer/` — `formatAnalysisToPrint()`
**How:** Vulnerabilities are decorated with commit author data. Analysis is decorated with formatting before output.

### 13. Registry/Map Pattern
**Where:** `controllers/analyzer/runner.go:detectVulnerabilityFuncs()`, `enums/images/images.go:MapValues()`
**How:** Maps associate languages with their corresponding formatter functions and Docker images.

### 14. Validation Pattern (Fluent)
**Where:** `usecases/cli/cli.go` — uses ozzo-validation library
**How:** `ValidateConfig()` chains validation rules with fluent API: `validation.ValidateStruct(cfg, validation.Field(...))`.

---

## 10. CI/CD Workflows

### Core Workflows
| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `test.yml` | push to main | Unit tests on ubuntu + windows (Go 1.22) |
| `lint.yml` | push to main | golangci-lint |
| `security.yml` | push to main | Run horusec against own codebase |
| `scanner-governance.yml` | weekly cron / manual | Auto-update scanner image versions |

### Release Workflows
| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `release-beta.yml` | manual (patch/minor/major) | Create beta release via GoReleaser |
| `release-rc.yml` | manual | Create release candidate |
| `release-final.yml` | manual | Create final release |

### Scanner Image Workflows (12)
| Workflow | Image |
|----------|-------|
| `update-horusec-c.yml` | `ghcr.io/heron-brito/horusec-c` |
| `update-horusec-csharp.yml` | `ghcr.io/heron-brito/horusec-csharp` |
| `update-horusec-elixir.yml` | `ghcr.io/heron-brito/horusec-elixir` |
| `update-horusec-generic.yml` | `ghcr.io/heron-brito/horusec-generic` |
| `update-horusec-go.yml` | `ghcr.io/heron-brito/horusec-go` |
| `update-horusec-hcl.yml` | `ghcr.io/heron-brito/horusec-hcl` |
| `update-horusec-js.yml` | `ghcr.io/heron-brito/horusec-js` |
| `update-horusec-leaks.yml` | `ghcr.io/heron-brito/horusec-leaks` |
| `update-horusec-php.yml` | `ghcr.io/heron-brito/horusec-php` |
| `update-horusec-python.yml` | `ghcr.io/heron-brito/horusec-python` |
| `update-horusec-ruby.yml` | `ghcr.io/heron-brito/horusec-ruby` |
| `update-horusec-shell.yml` | `ghcr.io/heron-brito/horusec-shell` |

All scanner workflows: `workflow_dispatch` with version input, GHCR login, `docker/build-push-action@v3`.

---

## 11. Formatter Pattern (How to add a new tool)

Each formatter follows this structure:
```
internal/services/formatters/<language>/<tool>/
    config.go         # Docker CMD template
    formatter.go      # IFormatter implementation
    formatter_test.go # Unit tests
    entities/         # Tool-specific output structs (optional)
    output.go         # Output parsing helpers (optional)
    deployments/      # Dockerfile for the language (shared)
```

Key method: `StartAnalysis(projectSubPath string)`:
1. Check if tool is ignored (`IsToolIgnore()`)
2. Execute container (`ExecuteContainer()`)
3. Parse tool output (`parseOutput()`)
4. Convert to Horusec vulnerabilities (`AddNewVulnerabilityIntoAnalysis()`)
5. Log finish (`LogAnalysisError()` / `LogDebugWithReplace()`)
