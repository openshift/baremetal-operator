/*
Copyright 2026 The Metal3 Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

func TestOpenShiftCRDs(t *testing.T) {
	names := []string{"baremetalswitches", "hostclaims", "hostdeploypolicies", "hostnetworkattachments"}
	for _, path := range []string{"kustomization.yaml", "../crd/ocp/ocp_kustomization.yaml"} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Resources []string `json:"resources"`
			}
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				found := slices.ContainsFunc(manifest.Resources, func(resource string) bool {
					return filepath.Base(resource) == "metal3.io_"+name+".yaml"
				})
				if !found {
					t.Errorf("missing CRD resource %s", name)
				}
			}
		})
	}

	file, err := os.Open("../render/capm3.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := k8syaml.NewYAMLOrJSONDecoder(file, 4096)
	found := map[string]bool{}
	for {
		var manifest struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata"`
		}
		if err := decoder.Decode(&manifest); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		if manifest.Kind != "CustomResourceDefinition" {
			continue
		}
		found[manifest.Name] = true
		for annotation, value := range map[string]string{
			"capability.openshift.io/name":                                "baremetal",
			"include.release.openshift.io/ibm-cloud-managed":              "true",
			"include.release.openshift.io/self-managed-high-availability": "true",
			"include.release.openshift.io/single-node-developer":          "true",
		} {
			if manifest.Annotations[annotation] != value {
				t.Errorf("CRD %s: annotation %s = %q, want %q", manifest.Name, annotation, manifest.Annotations[annotation], value)
			}
		}
	}
	for _, name := range names {
		if !found[name+".metal3.io"] {
			t.Errorf("missing rendered CRD %s", name)
		}
	}
}
