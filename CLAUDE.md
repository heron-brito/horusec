# Horusec - Claude Code Context

## Git Workflow Rules
- **Always work on a branch** prefixed with `fix-` (e.g., `fix-goreleaser-v2`, `fix-docker-auth`)
- **Never commit directly to `main`** — all changes must go through a Pull Request
- Stage only the changed files, commit, then create a PR from the current branch to `main` using `gh pr create`

## Project Overview
- **Horusec** - Open-source security scanning CLI tool written in Go
- Performs static code analysis for 18+ programming languages
- Uses 20+ security tools, scans Git history for secrets
- Can run standalone or via Docker containers
- Module: `github.com/ZupIT/horusec/` (Go 1.17)

## Key File Paths
- Entry point: `cmd/app/main.go` (Cobra CLI)
- Config: `config/config.go` (GlobalOptions, 25+ env vars, viper)
- Main analyzer: `internal/controllers/analyzer/analyzer.go`
- Controllers: `internal/controllers/`
- Services: `internal/services/` (docker, horusec_api)
- Entities: `internal/entities/`
- Usecases: `internal/usecases/`
- E2E tests: `e2e/`
- Deployment scripts: `deployments/scripts/`

## Architecture
- Clean Architecture / Hexagonal pattern
- Constructor-based DI (`New()` returning interfaces)
- Interface-driven design for testability
- CLI commands: `start`, `generate`, `version`

## Testing
- Unit tests: `make test` (`go test` with `-race -timeout=5m -parallel=1 -failfast -short`)
- E2E tests: `make test-e2e` (builds all tools first, 30min timeout)
- Coverage requirements: `cmd/` and `config/` = 90%, `internal/` = 86%
- Frameworks: testify, ginkgo/gomega BDD

## Build & Dev Commands
- `make lint` - golangci-lint
- `make test` - unit tests
- `make coverage` - coverage report
- `make build-dev` - dev build
- `make pipeline` - full CI (format, lint, test, coverage, security)
- Binaries built for: linux-amd64/arm64, darwin-amd64/arm64, windows

## Linting
- Config: `.golangci.yml`
- Local prefix: `github.com/ZupIT/horusec/`

## Security Tools (22 total)
Defined in `internal/entities/toolsconfig/tools_config.go`. All enabled by default.

### SAST
| Tool | Language/Target | Resumo |
|------|----------------|--------|
| Bandit | Python | Analisa código Python em busca de problemas de segurança comuns, como uso de funções inseguras, injeções e más configurações de criptografia. Gera relatórios por severidade e confiança, sendo amplamente usado em pipelines CI/CD para projetos Python. |
| Brakeman | Ruby (Rails) | Scanner estático focado em aplicações Rails que detecta vulnerabilidades como SQL injection, XSS, CSRF e uso inseguro de `eval`. É executado sem necessidade de rodar a aplicação, analisando diretamente o código-fonte. |
| Flawfinder | C/C++ | Examina código C/C++ procurando por chamadas a funções reconhecidamente perigosas (como `strcpy`, `gets`, `sprintf`) usando um banco de dados de riscos. Atribui níveis de risco e sugere alternativas mais seguras. |
| GoSec | Go | Realiza análise estática de código Go inspecionando a AST (Abstract Syntax Tree) e o SSA em busca de vulnerabilidades como SQL injection, credenciais hardcoded, uso inseguro de TLS e permissões de arquivo incorretas. |
| PhpCS | PHP | Detecta violações de padrões de codificação PHP e potenciais problemas de segurança, como uso de funções depreciadas, controle de erros inadequado e más práticas que podem levar a vulnerabilidades. |
| SecurityCodeScan | C#/.NET | Analisa código C# e VB.NET em busca de vulnerabilidades OWASP, incluindo SQL injection, XSS, CSRF, Path Traversal e uso incorreto de APIs criptográficas do .NET framework. |
| Semgrep | Multi-language | Motor de análise semântica baseado em padrões YAML que suporta dezenas de linguagens. Permite escrever regras customizadas que casam com padrões de código estrutural, sendo altamente extensível para políticas de segurança específicas. |
| Sobelow | Elixir/Phoenix | Scanner de segurança voltado para o framework Phoenix (Elixir) que detecta vulnerabilidades como injeção de código, XSS, CSRF, Directory Traversal e uso inseguro de configurações do framework. |
| HorusecEngine | Multi-language | Motor proprietário do Horusec que realiza análise baseada em expressões regulares e padrões semânticos. Cobre linguagens não suportadas pelas demais ferramentas e permite adicionar regras customizadas sem necessidade de ferramentas externas. |

### Dependency Scanners
| Tool | Language/Target | Resumo |
|------|----------------|--------|
| BundlerAudit | Ruby gems | Verifica o `Gemfile.lock` de projetos Ruby contra o banco de dados de vulnerabilidades do Ruby Advisory Database, identificando gems com CVEs conhecidos e sugerindo versões corrigidas. |
| MixAudit | Elixir | Audita dependências de projetos Elixir listadas no `mix.lock` contra o banco de dados de advisories do Elixir, alertando sobre pacotes com vulnerabilidades de segurança conhecidas. |
| NpmAudit | JavaScript/Node.js (npm) | Utiliza o comando `npm audit` para consultar o registro npm em busca de vulnerabilidades conhecidas nas dependências declaradas no `package-lock.json`, exibindo CVEs e níveis de severidade. |
| YarnAudit | JavaScript/Node.js (yarn) | Equivalente ao NpmAudit para projetos que usam Yarn como gerenciador de pacotes, consultando o advisory database do npm a partir do `yarn.lock` para identificar dependências vulneráveis. |
| Safety | Python (pip) | Verifica dependências Python listadas em `requirements.txt` ou instaladas no ambiente contra o Safety DB, um banco de dados curado de pacotes PyPI com vulnerabilidades conhecidas. |
| OwaspDependencyCheck | Multi-language | Ferramenta da OWASP que analisa dependências de múltiplas linguagens (Java, .NET, Node.js, Python, Ruby, etc.) identificando componentes com vulnerabilidades públicas no NVD (National Vulnerability Database). |
| DotnetCli | C#/.NET (NuGet) | Usa a CLI nativa do .NET (`dotnet list package --vulnerable`) para verificar pacotes NuGet referenciados no projeto contra o banco de dados de advisories do NuGet.org, sem necessidade de ferramentas externas. |
| Nancy | C#/.NET (NuGet) | Scanner de dependências NuGet que consulta o OSS Index da Sonatype para identificar pacotes com CVEs conhecidos, suportando projetos .NET Core e .NET Framework via análise do arquivo de dependências. |
| Trivy | Containers/OS packages | Scanner versátil da Aqua Security que detecta vulnerabilidades em imagens Docker, pacotes de SO (Alpine, Ubuntu, etc.) e dependências de aplicação. Também verifica misconfigurações em Dockerfiles e IaC. |

### Secret Detection
| Tool | Language/Target | Resumo |
|------|----------------|--------|
| GitLeaks | All languages / Git history | Varre repositórios Git em busca de segredos expostos (API keys, tokens, senhas, certificados) tanto no código atual quanto em todo o histórico de commits, usando regras de expressões regulares de alta precisão para evitar falsos positivos. |

### Infrastructure-as-Code
| Tool | Language/Target | Resumo |
|------|----------------|--------|
| Checkov | Terraform, CloudFormation, K8s, Dockerfile | Framework de análise estática para IaC da Bridgecrew que verifica centenas de políticas de segurança e compliance em Terraform, CloudFormation, Kubernetes, Dockerfiles e ARM templates, com suporte a políticas customizadas em Python ou YAML. |
| TfSec | Terraform | Scanner de segurança especializado em Terraform que detecta misconfigurações em provedores cloud (AWS, Azure, GCP) como buckets S3 públicos, grupos de segurança permissivos e ausência de criptografia, com regras baseadas em boas práticas CIS. |

### Script Analysis
| Tool | Language/Target | Resumo |
|------|----------------|--------|
| ShellCheck | Shell/Bash | Linter estático para scripts Shell e Bash que identifica bugs comuns, más práticas e potenciais vulnerabilidades de segurança como command injection, quoting incorreto e uso de variáveis não inicializadas. Amplamente adotado como padrão de qualidade para scripts de automação. |
