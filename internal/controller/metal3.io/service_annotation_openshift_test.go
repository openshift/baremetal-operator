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
	"testing"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner/fixture"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestServiceAnnotationWithServicing tests that service annotation triggers
// servicing without a reboot cycle.
func TestServiceAnnotationWithServicing(t *testing.T) {
	host := newDefaultHost(t)
	host.Annotations = make(map[string]string)
	host.Annotations[metal3api.ServiceAnnotationPrefix] = ""
	host.Status.PoweredOn = true
	host.Status.Provisioning.State = metal3api.StateProvisioned
	host.Spec.Online = true
	host.Spec.Image = &metal3api.Image{URL: "foo", Checksum: "123"}
	host.Spec.Firmware = &metal3api.FirmwareConfig{
		VirtualizationEnabled: ptr.To(true),
	}
	host.Status.Provisioning.Image.URL = "foo"

	hup := &metal3api.HostUpdatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      host.Name,
			Namespace: namespace,
		},
		Spec: metal3api.HostUpdatePolicySpec{
			FirmwareSettings: metal3api.HostUpdatePolicyOnReboot,
			FirmwareUpdates:  metal3api.HostUpdatePolicyOnReboot,
		},
	}

	fix := fixture.Fixture{PoweredOn: true}
	r := newTestReconcilerWithFixture(t, &fix, host, hup)

	// Servicing should start without powering off
	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			return host.Status.OperationalStatus == metal3api.OperationalStatusServicing
		},
	)

	// Host should remain powered on during servicing
	assert.True(t, host.Status.PoweredOn)

	// Servicing completes, annotation is cleared
	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			_, exists := host.Annotations[metal3api.ServiceAnnotationPrefix]
			return host.Status.OperationalStatus == metal3api.OperationalStatusOK && !exists
		},
	)

	// Host should still be powered on
	assert.True(t, host.Status.PoweredOn)
}

// TestServiceAnnotationWithoutHUP ensures servicing is not triggered by
// service annotation when no HostUpdatePolicy exists.
func TestServiceAnnotationWithoutHUP(t *testing.T) {
	host := newDefaultHost(t)
	host.Annotations = make(map[string]string)
	host.Annotations[metal3api.ServiceAnnotationPrefix] = ""
	host.Status.PoweredOn = true
	host.Status.Provisioning.State = metal3api.StateProvisioned
	host.Spec.Online = true
	host.Spec.Image = &metal3api.Image{URL: "foo", Checksum: "123"}
	host.Status.Provisioning.Image.URL = "foo"
	host.Spec.Firmware = &metal3api.FirmwareConfig{
		VirtualizationEnabled: ptr.To(true),
	}

	fix := fixture.Fixture{PoweredOn: true}
	r := newTestReconcilerWithFixture(t, &fix, host)

	// Without HUP, servicing should not start; annotation should be cleared
	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			if host.Status.OperationalStatus == metal3api.OperationalStatusServicing {
				t.Fatal("host entered servicing unexpectedly")
			}
			_, exists := host.Annotations[metal3api.ServiceAnnotationPrefix]
			return host.Status.OperationalStatus == metal3api.OperationalStatusOK && !exists
		},
	)

	assert.True(t, host.Status.PoweredOn)
}

// TestServiceAnnotationOnNonProvisionedHost ensures the service annotation
// is preserved (not silently swallowed) on hosts that are not yet provisioned.
func TestServiceAnnotationOnNonProvisionedHost(t *testing.T) {
	host := newDefaultHost(t)
	host.Annotations = make(map[string]string)
	host.Annotations[metal3api.ServiceAnnotationPrefix] = ""
	host.Status.PoweredOn = true
	host.Status.Provisioning.State = metal3api.StateAvailable
	host.Spec.Online = true

	fix := fixture.Fixture{PoweredOn: true}
	r := newTestReconcilerWithFixture(t, &fix, host)

	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			if host.Status.OperationalStatus == metal3api.OperationalStatusServicing {
				t.Fatal("host entered servicing unexpectedly")
			}
			return host.Status.OperationalStatus == metal3api.OperationalStatusOK
		},
	)

	// Annotation should survive on a non-provisioned host
	_, exists := host.Annotations[metal3api.ServiceAnnotationPrefix]
	assert.True(t, exists)
}

// TestServiceAnnotationSuffixed tests that suffixed service annotations
// also trigger servicing.
func TestServiceAnnotationSuffixed(t *testing.T) {
	host := newDefaultHost(t)
	host.Annotations = make(map[string]string)
	host.Annotations[metal3api.ServiceAnnotationPrefix+"/my-controller"] = ""
	host.Status.PoweredOn = true
	host.Status.Provisioning.State = metal3api.StateProvisioned
	host.Spec.Online = true
	host.Spec.Image = &metal3api.Image{URL: "foo", Checksum: "123"}
	host.Spec.Firmware = &metal3api.FirmwareConfig{
		VirtualizationEnabled: ptr.To(true),
	}
	host.Status.Provisioning.Image.URL = "foo"

	hup := &metal3api.HostUpdatePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      host.Name,
			Namespace: namespace,
		},
		Spec: metal3api.HostUpdatePolicySpec{
			FirmwareSettings: metal3api.HostUpdatePolicyOnReboot,
		},
	}

	fix := fixture.Fixture{PoweredOn: true}
	r := newTestReconcilerWithFixture(t, &fix, host, hup)

	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			return host.Status.OperationalStatus == metal3api.OperationalStatusServicing
		},
	)

	assert.True(t, host.Status.PoweredOn)

	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			return host.Status.OperationalStatus == metal3api.OperationalStatusOK
		},
	)

	// Suffixed annotation should NOT be cleared (only base is cleared by BMO)
	_, exists := host.Annotations[metal3api.ServiceAnnotationPrefix+"/my-controller"]
	assert.True(t, exists)
}
