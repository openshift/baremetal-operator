# Bare Metal Operator

The Bare Metal Operator implements a Kubernetes API for managing bare metal
hosts. It maintains an inventory of available hosts as instances of the
`BareMetalHost` Custom Resource Definition.

Please, see the [upstream project](https://github.com/metal3-io/baremetal-operator)
for more information.

## How to add a new upstream CRD to openshift

In [cluster-baremetal-operator](https://github.com/openshift/cluster-baremetal-operator),
update the BMO ClusterRole in
[`config/rbac/bmo_role.yaml`](https://github.com/openshift/cluster-baremetal-operator/blob/main/config/rbac/bmo_role.yaml).
Add only the verbs BMO needs for the new resource and its `status` and `finalizers`
subresources, if applicable. Do not add BMO permissions to CBO's Kubebuilder markers.

Then, regenerate the CBO release manifests:

```
make manifests
```

This PR to cluster-baremetal-operator will block your PR to bring the new CRDs to baremetal-operator,
unless the controller for the new API is feature-gated.
