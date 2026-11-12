#!/bin/bash

set -eu -o pipefail

mkdir -p manifests
# The default (for now) manifest with v1alpha1 only.
tools/bin/kustomize build openshift/crds/v1alpha1-only > \
    manifests/0000_31_cluster-baremetal-operator_00_baremetalhost.crd.yaml
# The new manifest with both API versions
tools/bin/kustomize build openshift/crds/v1beta1 > \
    manifests/0000_31_cluster-baremetal-operator_00_baremetalhost_with_v1beta1.crd.yaml

# Sanity checks
if grep cert-manager.io/inject-ca-from manifests/*.yaml; then
    echo "FAIL: upstream cert-manager annotation needs removal"
    exit 1
fi
if grep conversion manifests/*.yaml; then
    echo "FAIL: disable the conversion webhook until CBO configures it"
    exit 1
fi
if grep v1beta1 manifests/0000_31_cluster-baremetal-operator_00_baremetalhost.crd.yaml; then
    echo "FAIL: manifests created with openshift/crds/v1alpha1-only contain v1beta1"
    exit 1
fi
if grep baremetal-operator-system manifests/*.yaml; then
    echo "FAIL: manifests mention the upstream namespace; check for unrelated resources"
    exit 1
fi
