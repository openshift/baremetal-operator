# Merging Upstream to Downstream

* Upstream: https://github.com/metal3-io/baremetal-operator
* Downstream: https://github.com/openshift/baremetal-operator

When merging changes from upstream to downstream, follow these steps:

## 1. Preparation

Ensure your local repository is up to date by fetching all remote branches from
git.

Check the current branch. Here you need to make sure that:
- There are no local changes you may overwrite
- You are on the right branch that is based on the `main` branch of
  `openshift/baremetal-operator`
- You're on the latest commit in that branch.

Create a new branch for the merge named `merge-upstream-$(date +%Y-%m-%d)`.

## 2. Merge Upstream Changes

```bash
git merge --no-ff upstream/main
```

## 3. Handle Common Conflicts

**GitHub Workflows**: Downstream typically removes `.github/` directory files. If conflicts occur:
- Keep the downstream version (usually removal)
- Verify with `git diff downstream/main -- .github/`, this must return empty

**Downstream-Specific Changes**: Look for commit messages starting with:
- `DOWNSTREAM:` - Changes specific to OpenShift
- `OCPBUGS-*:` - OpenShift bug fixes

**OWNERS**: Revert any changes to `OWNERS`, you must keep the downstream
version of this file.

These must be preserved during the merge, unless replaced by an equivalent
upstream commit.

Commit the merge; the commit message should be "Merge upstream".

## 4. Downstream specific actions

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

Note that all dependencies must be vendored in OpenShift. You cannot rely on
downloading modules. Do not use `-mod=mod` with `go build`.

Commit the changes to `vendor` directories.  The commit message should be "Update vendor".

**New CRDs from Upstream**: Check if the upstream merge includes any new Custom Resource Definitions:

```bash
# Check for new CRD files
git diff upstream/main HEAD~1 --name-only | grep "apis/metal3.io/v1alpha1/*_types.go"

# Look for new CRD definitions in config/crd/bases/
git diff upstream/main HEAD~1 --name-only | grep "config/crd/bases/"
```

If new CRDs are found, you must follow the process documented in the [README](README.md#how-to-add-a-new-upstream-crd-to-openshift):

1. **First**: Ask the user to open a PR to [cluster-baremetal-operator](https://github.com/openshift/cluster-baremetal-operator) adding kubebuilder RBAC directives for the new CRD in `provisioning_controller.go`
2. **Then**: Add the new CRD to `config/crd/ocp/ocp_kustomization.yaml` in this repository

**Important**: The cluster-baremetal-operator PR must be merged before the baremetal-operator merge PR, as it's a blocking dependency.

## 5. Test the Changes

Before creating a PR, ensure:
- Code compiles: `make build`
- Tests pass: `make test`

You might run into bugs related to changes in Golang version.  Try to fix the bug and commit the changes.

## 6. Push

Push to the user's fork (replacing `origin` with the name of the user's
personal fork):

```bash
git push -u origin merge-upstream-$(date +%Y-%m-%d)
```

Tell the user how to create the pull request using the base repository
`openshift/baremetal-operator`.  Ask them if they want to open the New Pull
Request page on GitHub:
`https://github.com/openshift/baremetal-operator/compare/main...<user
fork>:baremetal-operator:<merge branch>`.
