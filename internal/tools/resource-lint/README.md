# Resource Linter

The resource linter tool is an AzureRM Provider code linting tool, specifically tailored for checking if the code is consistent with rules defined in `/contributing`.

## Quick Usage

```bash
# From the repository root directory
cd /path/to/terraform-provider-azurerm

# Use make (recommended)
make resource-lint

# Or install and run directly
cd internal/tools/resource-lint && go install .
resource-lint ./internal/services/...
```

## Lint Checks

For additional information about each check, see the documentation in passes's directory (e.g., `passes/doc.go`).

### Azure Best Practice Checks

| Check | Description |
|-------|-------------|
| AZBP001 | check for all String arguments have `ValidateFunc` |
| AZBP002 | check for `Optional+Computed` fields follow conventions |
| AZBP003 | check for `pointer.ToEnum` to convert Enum type instead of explicitly type conversion |
| AZBP004 | check for zero-value initialization followed by nil check and pointer dereference that should use `pointer.From` | 

### Azure New Resource Checks

| Check | Description |
|-------|-------------|
| AZNR001 | check for Schema field ordering |

### Azure Naming Rule Checks

| Check | Description |
|-------|-------------|
| AZRN001 | check for percentage properties use `_percentage` suffix instead of `_in_percent` |

### Azure Resource Error Checks

| Check | Description |
|-------|-------------|
| AZRE001 | check for fixed error strings using `fmt.Errorf` instead of `errors.New` |

### Azure Schema Design Checks

| Check | Description |
|-------|-------------|
| AZSD001 | check for `MaxItems:1` blocks with single property should be flattened |
| AZSD002 | check for `AtLeastOneOf` validation on TypeList fields with all optional nested fields |

## Installation

This tool is part of the terraform-provider-azurerm repository with its own go.mod for dependency isolation.

```bash
# Navigate to the provider repository
cd /path/to/terraform-provider-azurerm

# The tool will be built automatically when run via make or scripts
```

## Usage

### Quick Start

```bash
# Run from the repository root
cd /path/to/terraform-provider-azurerm

# Check all services (default)
make resource-lint

# Install once, then use anywhere
cd internal/tools/resource-lint && go install .

# Check specific packages
resource-lint ./internal/services/compute/...

# Check from diff file
resource-lint --diff=changes.txt ./internal/services/...

# Check all code without filtering
resource-lint --no-filter ./internal/services/...

# List all available checks
resource-lint --list
```

### Integration with Makefile

The tool is integrated into the project's Makefile:

```makefile
# GNUmakefile
resource-lint:
	@echo "==> Installing resource-lint..."
	@cd internal/tools/resource-lint && go install .
	@echo "==> Running resource linter..."
	@resource-lint ./internal/services/...
```

The binary is installed to `$GOPATH/bin` and can be used directly after installation.

Run via make:
```bash
make resource-lint
```

### Common Options

```bash
--remote=<name>    # Specify git remote (origin/upstream)
--base=<branch>    # Specify base branch
--diff=<file>      # Read diff from file
--no-filter        # Analyze all lines (not just changes)
--list             # List all available checks
--help             # Show help
```

**Important Notes**: 
- **Run from repository root**: All commands must be run from the repository root (where `.git` exists)
- Package paths are relative to the repository root (e.g., `./internal/services/...`)
- By default, only changed lines are analyzed (use `--no-filter` to check all code)

### Troubleshooting

**Error: "not in a git repository"**
```bash
# Make sure you're in the repository root
cd /path/to/terraform-provider-azurerm
make resource-lint
```

**To check specific packages**
```bash
go run ./internal/tools/resource-lint ./internal/services/compute/...
```

**To check all code without filtering**
```bash
go run ./internal/tools/resource-lint --no-filter ./internal/services/...
```
