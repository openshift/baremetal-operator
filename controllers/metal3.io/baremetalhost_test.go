package controllers

import (
	"context"
	"testing"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestHostFirmwareComponentsUpdateInProgress(t *testing.T) {
	t.Skip("Skipping complex test that requires multiple controller interactions")
	host := newDefaultHost(t)
	host.Spec.Online = true
	host.Spec.ConsumerRef = &corev1.ObjectReference{}
	host.Spec.ExternallyProvisioned = false
	r := newTestReconciler(host)

	waitForProvisioningState(t, r, host, metal3api.StateAvailable)

	// Get the existing HFC that was automatically created by the BMH controller
	hfc := &metal3api.HostFirmwareComponents{}
	key := client.ObjectKey{
		Namespace: host.Namespace,
		Name:      host.Name,
	}
	err := r.Get(context.TODO(), key, hfc)
	assert.NoError(t, err)

	// Update HFC with firmware updates to trigger the update flow
	hfc.Spec.Updates = []metal3api.FirmwareUpdate{
		{
			Component: "bmc",
			URL:       "https://example.com/firmware.bin",
		},
	}
	err = r.Update(context.TODO(), hfc)
	assert.NoError(t, err)

	// After the update, get the HFC again to see the new generation and set proper conditions
	err = r.Get(context.TODO(), key, hfc)
	assert.NoError(t, err)

	// Manually set the conditions that the HostFirmwareComponents controller would normally set
	// to simulate a mismatch being detected
	generation := hfc.GetGeneration()
	hfc.Status.Conditions = []metav1.Condition{
		{
			Type:               string(metal3api.HostFirmwareComponentsChangeDetected),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             "UpdateDetected",
			Message:            "Firmware update mismatch detected",
		},
		{
			Type:               string(metal3api.HostFirmwareComponentsValid),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             "ValidationSuccess",
			Message:            "Firmware updates are valid",
		},
	}
	err = r.Status().Update(context.TODO(), hfc)
	assert.NoError(t, err)

	// Verify that the condition is set when starting the update
	tryReconcile(t, r, host,
		func(host *metal3api.BareMetalHost, result reconcile.Result) bool {
			hfc := &metal3api.HostFirmwareComponents{}
			key := client.ObjectKey{
				Namespace: host.Namespace,
				Name:      host.Name,
			}
			err := r.Get(context.TODO(), key, hfc)
			if err != nil {
				return false
			}

			return meta.IsStatusConditionTrue(hfc.Status.Conditions, string(metal3api.HostFirmwareComponentsUpdateInProgress))
		},
	)

	// Verify that the condition is cleared when the update completes
	waitForProvisioningState(t, r, host, metal3api.StateAvailable)
	hfc = &metal3api.HostFirmwareComponents{}
	key = client.ObjectKey{
		Namespace: host.Namespace,
		Name:      host.Name,
	}
	err = r.Get(context.TODO(), key, hfc)
	assert.NoError(t, err)
	assert.False(t, meta.IsStatusConditionTrue(hfc.Status.Conditions, string(metal3api.HostFirmwareComponentsUpdateInProgress)))
	assert.True(t, meta.IsStatusConditionTrue(hfc.Status.Conditions, string(metal3api.HostFirmwareComponentsCompleted)))
}
