/*

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

package controllers

import (
	"strings"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
)

// hasServiceAnnotation returns true if the host has any service annotation
// (base or suffixed).
func hasServiceAnnotation(info *reconcileInfo) bool {
	for annotation := range info.host.GetAnnotations() {
		if isServiceAnnotation(annotation) {
			return true
		}
	}
	return false
}

// isServiceAnnotation returns true if the provided annotation is a service
// annotation (either suffixed or not).
func isServiceAnnotation(annotation string) bool {
	return strings.HasPrefix(annotation, metal3api.ServiceAnnotationPrefix+"/") || annotation == metal3api.ServiceAnnotationPrefix
}

// clearServiceAnnotations deletes the base service annotation if it exists
// on the provided host.
func clearServiceAnnotations(host *metal3api.BareMetalHost) bool {
	if _, exists := host.Annotations[metal3api.ServiceAnnotationPrefix]; exists {
		delete(host.Annotations, metal3api.ServiceAnnotationPrefix)
		return true
	}
	return false
}
