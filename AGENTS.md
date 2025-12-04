# Baremetal Operator Architecture

## Overview

The Baremetal Operator (BMO) is a Kubernetes operator designed to manage bare metal hosts in a Kubernetes cluster. It provides a declarative way to manage physical hardware, enabling automated provisioning, configuration, and lifecycle management of bare metal servers through integration with OpenStack Ironic.

## Main Controllers

The operator contains several key controllers, each responsible for managing different aspects of bare metal infrastructure:

### 1. BareMetalHost Controller

**Purpose**: Primary controller for managing physical servers

**Responsibilities**:
- Host provisioning and deprovisioning
- State management and reconciliation
- Integration with provisioner backends (primarily Ironic)
- Power management and BMC interactions
- Image deployment to physical hosts

**Location**: `internal/controller/metal3.io/baremetalhost_controller.go`

### 2. PreprovisioningImage Controller

**Purpose**: Manages preprovisioning images for hosts

**Responsibilities**:
- Creates and manages preprovisioning images
- Handles image lifecycle
- Optional component (enabled with `--enable-preprovisioningimage-controller` flag)

**Location**: `internal/controller/metal3.io/preprovisioningimage_controller.go`

### 3. HostFirmwareSettings Controller

**Purpose**: Manages firmware configuration for bare metal hosts

**Responsibilities**:
- Handles firmware settings and updates
- Synchronizes firmware configuration with BMC
- Tracks firmware setting changes

**Location**: `internal/controller/metal3.io/hostfirmwaresettings_controller.go`

### 4. HostFirmwareComponents Controller

**Purpose**: Manages firmware components for hosts

**Responsibilities**:
- Tracks firmware component information
- Handles firmware component updates
- Maintains firmware inventory

**Location**: `internal/controller/metal3.io/hostfirmwarecomponents_controller.go`

### 5. BMCEventSubscription Controller

**Purpose**: Manages event subscriptions from Baseboard Management Controllers

**Responsibilities**:
- Handles hardware-related event monitoring
- Manages BMC event subscriptions
- Processes hardware events

**Location**: `internal/controller/metal3.io/bmceventsubscription_controller.go`

### 6. DataImage Controller

**Purpose**: Manages data images associated with hosts

**Responsibilities**:
- Handles data image lifecycle
- Manages image-related operations
- Coordinates image deployment

**Location**: `internal/controller/metal3.io/dataimage_controller.go`

## Custom Resource Definitions (CRDs)

The operator manages the following Custom Resource Definitions in the `metal3.io` API group:

1. **BareMetalHost** - Represents a physical server with its configuration and state
2. **PreprovisioningImage** - Manages preprovisioning images for initial host setup
3. **HostFirmwareSettings** - Defines BIOS/firmware settings for hosts
4. **HostFirmwareComponents** - Tracks firmware components and versions
5. **BMCEventSubscription** - Manages BMC event subscriptions for monitoring
6. **DataImage** - Represents data images for host configuration
7. **HardwareData** - Contains hardware inventory information
8. **FirmwareSchema** - Defines firmware configuration schemas

## Provisioner Architecture

The operator supports multiple provisioning backends:

### Ironic Provisioner (Default/Production)

- Uses OpenStack Ironic for actual hardware provisioning
- Supports full hardware lifecycle management
- Configurable via environment variables
- Handles power management, introspection, and deployment

### Test Provisioner

- Disables Ironic communication
- Used for testing without real hardware
- Enabled with `--test-mode` flag

### Demo Provisioner

- Simulates host state management
- Used for demonstration and development
- Provides fake state transitions

## Key Components

### Webhooks

The operator implements validating and mutating webhooks for:
- Resource validation before storage
- Default value injection
- Cross-field validation
- Immutability enforcement

### Metrics

- Exposes Prometheus metrics on port `:8443`
- Tracks operational metrics and controller performance
- Monitors reconciliation success/failure rates

### Health Checks

- Readiness probe on port `:9440`
- Health check endpoints
- Leader election status

### Leader Election

- Supports high availability deployment
- Multiple replicas with leader election
- Configurable lease duration and renewal

## Entry Point

**Main Entry Point**: `main.go`

The main function:
1. Parses command-line flags and environment variables
2. Configures logging and metrics
3. Sets up the controller manager
4. Registers controllers based on configuration
5. Configures webhooks
6. Starts the manager and begins reconciliation

## Configuration Options

### Environment Variables

- `DEPLOY_KERNEL_URL` - URL for deployment kernel
- `DEPLOY_RAMDISK_URL` - URL for deployment ramdisk
- `IRONIC_ENDPOINT` - Ironic API endpoint
- `IRONIC_INSPECTOR_ENDPOINT` - Ironic Inspector endpoint
- `BMO_CONCURRENCY` - Number of concurrent reconciles

### Command-Line Flags

- `--namespace` - Namespace to watch (empty for all namespaces)
- `--enable-leader-election` - Enable leader election for HA
- `--test-mode` - Enable test provisioner mode
- `--dev-mode` - Enable development mode features
- `--webhook-port` - Port for webhook server

## Deployment Considerations

### Namespace Modes

- **Cluster-wide**: Watches all namespaces (default when namespace is empty)
- **Namespace-scoped**: Watches a single namespace
- **Multi-namespace**: Can be configured to watch specific namespaces

### High Availability

- Supports multiple replicas with leader election
- Only the leader performs reconciliation
- Automatic failover on leader failure

### Security

- TLS configuration for webhooks
- RBAC permissions for Kubernetes resources
- Ironic authentication support

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes API                     │
└─────────────────────┬───────────────────────────────┘
                      │
                      │ Watch/Update CRDs
                      │
┌─────────────────────▼───────────────────────────────┐
│         Baremetal Operator Manager                  │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  BareMetalHost Controller                     │  │
│  └───────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────┐  │
│  │  HostFirmwareSettings Controller              │  │
│  └───────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────┐  │
│  │  Other Controllers...                         │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  Provisioner Interface                        │  │
│  └─────────────────────┬─────────────────────────┘  │
└─────────────────────────┼───────────────────────────┘
                          │
              ┌───────────┴───────────┐
              │                       │
      ┌───────▼────────┐    ┌─────────▼────────┐
      │ Ironic         │    │ Test/Demo        │
      │ Provisioner    │    │ Provisioners     │
      └───────┬────────┘    └──────────────────┘
              │
      ┌───────▼────────┐
      │ OpenStack      │
      │ Ironic API     │
      └───────┬────────┘
              │
      ┌───────▼────────┐
      │ Bare Metal     │
      │ Hosts (BMC)    │
      └────────────────┘
```

## Upstream and Downstream Relationship

This repository maintains both an upstream (metal3-io) and downstream (OpenShift) version of the baremetal-operator. Understanding this relationship is crucial for development and maintenance.

### Repository Setup

The repository has multiple git remotes configured:

- **upstream**: https://github.com/metal3-io/baremetal-operator (main development)
- **downstream**: https://github.com/openshift/baremetal-operator (OpenShift fork)
- **origin**: Your personal fork

### Merging Upstream to Downstream

When merging changes from upstream to downstream, follow these steps:

#### 1. Preparation

Ensure your local repository is up to date:

```bash
# Fetch all remotes
git fetch --all

# Check current branch
git status

# Create a new branch for the merge
git checkout -b merge-upstream-$(date +%Y-%m-%d)
```

#### 2. Merge Upstream Changes

```bash
# Merge upstream/main into your branch (no fast-forward)
git merge --no-ff upstream/main

# Note: You may encounter merge conflicts, particularly in:
# - .github/ workflows (downstream removes these)
# - go.mod and go.sum (version differences)
# - Vendored dependencies
# - OpenShift-specific customizations
```

#### 3. Handle Common Conflicts

**GitHub Workflows**: Downstream typically removes `.github/` directory files. If conflicts occur:
- Keep the downstream version (usually removal)
- Verify with `git diff downstream/main -- .github/`

**Go Dependencies**: After merging, update dependencies:
```bash
# Clean up and tidy all go.mod files across all modules
make mod

# Update vendor directories for all modules
make vendor

# These commands handle:
# - Root module (go.mod)
# - apis/ module
# - pkg/hardwareutils/ module
# - hack/tools/ module
# - test/ module
```

**Downstream-Specific Changes**: Look for commit messages starting with:
- `DOWNSTREAM:` - Changes specific to OpenShift
- `OCPBUGS-*:` - OpenShift bug fixes

These should be preserved during the merge.

**New CRDs from Upstream**: Check if the upstream merge includes any new Custom Resource Definitions:

```bash
# Check for new CRD files
git diff upstream/main HEAD~1 --name-only | grep "apis/.*_types.go"

# Look for new CRD definitions in config/crd/bases/
git diff upstream/main HEAD~1 --name-only | grep "config/crd/bases/"
```

If new CRDs are found, you must follow the process documented in the [README](README.md#how-to-add-a-new-upstream-crd-to-openshift):

1. **First**: Create a PR to [cluster-baremetal-operator](https://github.com/openshift/cluster-baremetal-operator) adding kubebuilder RBAC directives in `provisioning_controller.go`
2. **Then**: Add the new CRD to `config/crd/ocp/ocp_kustomization.yaml` in this repository

**Important**: The cluster-baremetal-operator PR must be merged before the baremetal-operator merge PR, as it's a blocking dependency.

#### 4. Verify the Merge

```bash
# Check what was merged
git log --oneline --graph upstream/main..HEAD

# Review differences from downstream main
git diff downstream/main

# Ensure no downstream-specific changes were lost
git log --oneline --grep="DOWNSTREAM\|OCPBUGS"
```

#### 5. Test the Changes

Before creating a PR, ensure:
- Code compiles: `make build`
- Tests pass: `make test`
- Manifests are up to date: `make manifests`
- Code is properly formatted: `make fmt`
- Linting passes: `make lint`

#### 6. Commit and Push

```bash
# If there were conflicts, they should be resolved and staged
# Commit the merge
git commit -m "Merge upstream

Merging changes from upstream metal3-io/baremetal-operator
as of $(git rev-parse --short upstream/main)

$(git log --oneline upstream/main ^HEAD~1 | head -10)
"

# Push to your fork
git push origin merge-upstream-$(date +%Y-%m-%d)
```

#### 7. Create Pull Request

Create a PR from your branch to `downstream/main` (or the appropriate downstream branch):

1. Title: `Merge upstream`
2. Description should include:
   - Upstream commit range being merged
   - Notable changes from upstream
   - Any conflicts resolved
   - Testing performed
   - Links to relevant upstream PRs

#### 8. Post-Merge Checklist

After the merge PR is approved and merged:

- Verify downstream CI passes
- If new CRDs were added, ensure the cluster-baremetal-operator PR was merged first
- Check for any downstream-specific functionality that may need updates
- Update any downstream-only documentation
- Monitor for issues in downstream deployments

### Key Differences Between Upstream and Downstream

Understanding the differences helps avoid conflicts:

1. **GitHub Actions**: Downstream uses OpenShift CI instead of GitHub Actions
   - `.github/` directory is typically absent or minimal in downstream

2. **Build Configuration**:
   - `.ci-operator.yaml` is downstream-specific
   - Dockerfile may have OpenShift-specific modifications

3. **Dependencies**:
   - May use different versions for OpenShift compatibility
   - Additional downstream-only dependencies possible

4. **Features**:
   - Some features may be disabled or modified in downstream
   - OpenShift-specific customizations marked with `DOWNSTREAM:` commits

5. **Testing**:
   - Different CI systems (GitHub Actions vs OpenShift CI)
   - May have additional or modified tests

### Automation Considerations

When using AI agents for merging:

1. **Always review conflicts manually** - Don't auto-resolve complex conflicts
2. **Preserve DOWNSTREAM commits** - These are intentional divergences
3. **Run `make mod && make vendor`** - After any go.mod changes
4. **Test thoroughly** - Especially OpenShift-specific functionality
5. **Document the merge** - Include commit ranges and notable changes
6. **Check for removed files** - GitHub workflows should stay removed
7. **Verify vendor/ directory** - Should be updated via `make vendor`, not manually

### Common Pitfalls

- **Don't** force-push over merge conflicts without review
- **Don't** blindly accept all upstream changes in conflicted files
- **Don't** forget to run `make mod && make vendor` after go.mod changes
- **Don't** remove DOWNSTREAM-prefixed commits
- **Don't** manually edit vendor/ directories (use `make vendor` instead)
- **Do** check that downstream-specific OpenShift CI configuration is preserved
- **Do** verify that removed GitHub Actions workflows stay removed
- **Do** test the build and basic functionality after merge

## Related Resources

- [Upstream Project](https://github.com/metal3-io/baremetal-operator)
- [Downstream Project](https://github.com/openshift/baremetal-operator)
- [Metal3.io Documentation](https://metal3.io)
- [API Documentation](docs/api.md)
- [Development Setup](docs/dev-setup.md)
- [BareMetalHost States](docs/baremetalhost-states.md)
- [Configuration Guide](docs/configuration.md)
