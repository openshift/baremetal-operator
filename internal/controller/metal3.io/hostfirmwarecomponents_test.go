package controllers

import (
	"context"
	"testing"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/baremetal-operator/pkg/hardwareutils/bmc"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Test support for HostFirmwareComponents in the HostFirmwareComponentsReconciler.
func getTestHFCReconciler(host *metal3api.HostFirmwareComponents) *HostFirmwareComponentsReconciler {
	c := fakeclient.NewClientBuilder().WithRuntimeObjects(host).WithStatusSubresource(host).Build()

	reconciler := &HostFirmwareComponentsReconciler{
		Client: c,
		Log:    ctrl.Log.WithName("test_reconciler").WithName("HostFirmwareComponents"),
	}

	return reconciler
}

func getMockHFCProvisioner(host *metal3api.BareMetalHost, components []metal3api.FirmwareComponentStatus) provisioner.Provisioner {
	state := fixture.Fixture{
		HostFirmwareComponents: fixture.HostFirmwareComponentsMock{
			Components: components,
		},
	}
	p, _ := state.NewProvisioner(context.TODO(), provisioner.BuildHostData(*host, bmc.Credentials{}),
		func(reason, message string) {})
	return p
}

// Mock components to return from provisioner.
func getCurrentComponents(updatedComponents string) []metal3api.FirmwareComponentStatus {
	var components []metal3api.FirmwareComponentStatus
	switch updatedComponents {
	case "bmc":
		components = []metal3api.FirmwareComponentStatus{
			{
				Component:          "bmc",
				InitialVersion:     "1.0.0",
				CurrentVersion:     "1.1.0",
				LastVersionFlashed: "1.1.0",
			},
			{
				Component:      "bios",
				InitialVersion: "1.0.1",
				CurrentVersion: "1.0.1",
			},
		}
	case "bios":
		components = []metal3api.FirmwareComponentStatus{
			{
				Component:      "bmc",
				InitialVersion: "1.0.0",
				CurrentVersion: "1.0.0",
			},
			{
				Component:          "bios",
				InitialVersion:     "1.0.1",
				CurrentVersion:     "1.1.10",
				LastVersionFlashed: "1.1.10",
			},
		}
	default:
		components = []metal3api.FirmwareComponentStatus{
			{
				Component:          "bmc",
				InitialVersion:     "1.0.0",
				CurrentVersion:     "1.1.0",
				LastVersionFlashed: "1.1.0",
			},
			{
				Component:          "bios",
				InitialVersion:     "1.0.1",
				CurrentVersion:     "1.1.10",
				LastVersionFlashed: "1.1.10",
			},
		}
	}

	return components
}

// Create the baremetalhost reconciler and use that to create bmh in same namespace.
func createBaremetalHostHFC() *metal3api.BareMetalHost {
	bmh := &metal3api.BareMetalHost{}
	bmh.ObjectMeta = metav1.ObjectMeta{Name: hostName, Namespace: hostNamespace}
	c := fakeclient.NewFakeClient(bmh)

	reconciler := &BareMetalHostReconciler{
		Client:             c,
		ProvisionerFactory: nil,
		Log:                ctrl.Log.WithName("bmh_reconciler").WithName("BareMetalHost"),
	}
	_ = reconciler.Create(context.TODO(), bmh)

	return bmh
}

// Create and HFC with input spec components.
func getHFC(spec metal3api.HostFirmwareComponentsSpec) *metal3api.HostFirmwareComponents {
	hfc := &metal3api.HostFirmwareComponents{}

	hfc.Status = metal3api.HostFirmwareComponentsStatus{
		Components: []metal3api.FirmwareComponentStatus{
			{
				Component:      "bmc",
				InitialVersion: "1.0.0",
				CurrentVersion: "1.0.0",
			},
			{
				Component:      "bios",
				InitialVersion: "1.0.1",
				CurrentVersion: "1.0.1",
			},
		},
	}

	hfc.TypeMeta = metav1.TypeMeta{
		Kind:       "HostFirmwareComponents",
		APIVersion: "metal3.io/v1alpha1"}
	hfc.ObjectMeta = metav1.ObjectMeta{
		Name:      hostName,
		Namespace: hostNamespace}

	hfc.Spec = spec
	return hfc
}

// Test the hostfirmwarecomponents reconciler functions.
func TestStoreHostFirmwareComponents(t *testing.T) {
	testCases := []struct {
		Scenario           string
		UpdatedComponents  string
		CurrentHFCResource *metal3api.HostFirmwareComponents
		ExpectedComponents *metal3api.HostFirmwareComponents
	}{
		{
			Scenario:          "update bmc",
			UpdatedComponents: "bmc",
			CurrentHFCResource: &metal3api.HostFirmwareComponents{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HostFirmwareComponents",
					APIVersion: "metal3.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "hostName",
					Namespace:       "hostNamespace",
					ResourceVersion: "1"},
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:          "bmc",
							InitialVersion:     "1.0.0",
							CurrentVersion:     "1.1.0",
							LastVersionFlashed: "1.1.0",
						},
						{
							Component:      "bios",
							InitialVersion: "1.0.1",
							CurrentVersion: "1.0.1",
						},
					},
				},
			},
			ExpectedComponents: &metal3api.HostFirmwareComponents{
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
					},
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:          "bmc",
							InitialVersion:     "1.0.0",
							CurrentVersion:     "1.1.0",
							LastVersionFlashed: "1.1.0",
						},
						{
							Component:      "bios",
							InitialVersion: "1.0.1",
							CurrentVersion: "1.0.1",
						},
					},
					Conditions: []metav1.Condition{
						{Type: "ChangeDetected", Status: "True", Reason: "OK"},
						{Type: "Valid", Status: "True", Reason: "OK"},
					},
				},
			},
		},
		{
			Scenario:          "update bios",
			UpdatedComponents: "bios",
			CurrentHFCResource: &metal3api.HostFirmwareComponents{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HostFirmwareComponents",
					APIVersion: "metal3.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "hostName",
					Namespace:       "hostNamespace",
					ResourceVersion: "1"},
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:      "bmc",
							InitialVersion: "1.0.0",
							CurrentVersion: "1.0.0",
						},
						{
							Component:          "bios",
							InitialVersion:     "1.0.1",
							CurrentVersion:     "1.1.10",
							LastVersionFlashed: "1.1.10",
						},
					},
				},
			},
			ExpectedComponents: &metal3api.HostFirmwareComponents{
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:      "bmc",
							InitialVersion: "1.0.0",
							CurrentVersion: "1.0.0",
						},
						{
							Component:          "bios",
							InitialVersion:     "1.0.1",
							CurrentVersion:     "1.1.10",
							LastVersionFlashed: "1.1.10",
						},
					},
					Conditions: []metav1.Condition{
						{Type: "ChangeDetected", Status: "True", Reason: "OK"},
						{Type: "Valid", Status: "True", Reason: "OK"},
					},
				},
			},
		},
		{
			Scenario:          "update all",
			UpdatedComponents: "all",
			CurrentHFCResource: &metal3api.HostFirmwareComponents{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HostFirmwareComponents",
					APIVersion: "metal3.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "hostName",
					Namespace:       "hostNamespace",
					ResourceVersion: "1"},
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:          "bmc",
							InitialVersion:     "1.0.0",
							CurrentVersion:     "1.1.0",
							LastVersionFlashed: "1.1.0",
						},
						{
							Component:          "bios",
							InitialVersion:     "1.0.1",
							CurrentVersion:     "1.1.10",
							LastVersionFlashed: "1.1.10",
						},
					},
				},
			},
			ExpectedComponents: &metal3api.HostFirmwareComponents{
				Spec: metal3api.HostFirmwareComponentsSpec{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
				},
				Status: metal3api.HostFirmwareComponentsStatus{
					Updates: []metal3api.FirmwareUpdate{
						{
							Component: "bmc",
							URL:       "https://myurl/newbmcfirmware",
						},
						{
							Component: "bios",
							URL:       "https://myurl/newbiosfirmware",
						},
					},
					Components: []metal3api.FirmwareComponentStatus{
						{
							Component:          "bmc",
							InitialVersion:     "1.0.0",
							CurrentVersion:     "1.1.0",
							LastVersionFlashed: "1.1.0",
						},
						{
							Component:          "bios",
							InitialVersion:     "1.0.1",
							CurrentVersion:     "1.1.10",
							LastVersionFlashed: "1.1.10",
						},
					},
					Conditions: []metav1.Condition{
						{Type: "ChangeDetected", Status: "True", Reason: "OK"},
						{Type: "Valid", Status: "True", Reason: "OK"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Scenario, func(t *testing.T) {
			ctx := t.Context()

			tc.ExpectedComponents.TypeMeta = metav1.TypeMeta{
				Kind:       "HostFirmwareComponents",
				APIVersion: "metal3.io/v1alpha1"}
			tc.ExpectedComponents.ObjectMeta = metav1.ObjectMeta{
				Name:            hostName,
				Namespace:       hostNamespace,
				ResourceVersion: "2"}

			hfc := tc.CurrentHFCResource
			r := getTestHFCReconciler(hfc)
			// Create a bmh resource needed by hfc reconciler
			bmh := createBaremetalHostHFC()

			prov := getMockHFCProvisioner(bmh, getCurrentComponents(tc.UpdatedComponents))

			info := &rhfcInfo{
				log: logf.Log.WithName("controllers").WithName("HostFirmwareComponents"),
				hfc: tc.CurrentHFCResource,
				bmh: bmh,
			}

			components, err := prov.GetFirmwareComponents()
			require.NoError(t, err)

			err = r.updateHostFirmware(info, components)
			require.NoError(t, err)

			// Check that resources get created or updated
			key := client.ObjectKey{
				Namespace: hfc.ObjectMeta.Namespace, Name: hfc.ObjectMeta.Name}
			actual := &metal3api.HostFirmwareComponents{}
			err = r.Client.Get(ctx, key, actual)
			require.NoError(t, err)

			assert.Equal(t, tc.ExpectedComponents.Spec.Updates, actual.Spec.Updates)
			assert.Equal(t, tc.ExpectedComponents.Status.Components, actual.Status.Components)

			currentTime := metav1.Now()
			tc.ExpectedComponents.Status.LastUpdated = &currentTime
			actual.Status.LastUpdated = &currentTime
			for i := range tc.ExpectedComponents.Status.Conditions {
				tc.ExpectedComponents.Status.Conditions[i].LastTransitionTime = currentTime
				actual.Status.Conditions[i].LastTransitionTime = currentTime
			}
			assert.Equal(t, tc.ExpectedComponents.Status.LastUpdated, actual.Status.LastUpdated)
			assert.Equal(t, tc.ExpectedComponents.Status.Conditions, actual.Status.Conditions)
		})
	}
}

// Test the function to validate the components in the Spec.
func TestValidadeHostFirmwareComponents(t *testing.T) {
	testCases := []struct {
		Scenario       string
		SpecUpdates    metal3api.HostFirmwareComponentsSpec
		ExpectedErrors []string
	}{
		{
			Scenario: "valid spec - all components",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "bmc", URL: "https://myurl/mybmcfw"},
					{Component: "bios", URL: "https://myurl/mybiosfw"},
				},
			},
			ExpectedErrors: []string{""},
		},
		{
			Scenario: "valid spec - only bios",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "bios", URL: "https://myurl/mybiosfw"},
				},
			},
			ExpectedErrors: []string{""},
		},
		{
			Scenario: "valid spec - only bmc",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "bmc", URL: "https://myurl/mybmcfw"},
				},
			},
			ExpectedErrors: []string{""},
		},
		{
			Scenario: "valid spec - with nic components",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "bmc", URL: "https://myurl/mybmcfw"},
					{Component: "nic:NIC.1", URL: "https://myurl/mynicfw"},
					{Component: "nic:AD007", URL: "https://myurl/mynic2fw"},
				},
			},
			ExpectedErrors: []string{""},
		},
		{
			Scenario: "invalid something component",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "something", URL: "https://myurl/myfw"},
				},
			},
			ExpectedErrors: []string{"component something is invalid, only 'bmc', 'bios', or names starting with 'nic:' are allowed as update names"},
		},
		{
			Scenario: "invalid something component with other valid components",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "bmc", URL: "https://myurl/mybmcfw"},
					{Component: "bios", URL: "https://myurl/mybiosfw"},
					{Component: "something", URL: "https://myurl/myfw"},
				},
			},
			ExpectedErrors: []string{"component something is invalid, only 'bmc', 'bios', or names starting with 'nic:' are allowed as update names"},
		},
		{
			Scenario: "component not in lowercase",
			SpecUpdates: metal3api.HostFirmwareComponentsSpec{
				Updates: []metal3api.FirmwareUpdate{
					{Component: "BMC", URL: "https://myurl/mybmcfw"},
					{Component: "BIOS", URL: "https://myurl/mybiosfw"},
				},
			},
			ExpectedErrors: []string{
				"component BMC is invalid, only 'bmc', 'bios', or names starting with 'nic:' are allowed as update names",
				"component BIOS is invalid, only 'bmc', 'bios', or names starting with 'nic:' are allowed as update names",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Scenario, func(t *testing.T) {
			ctx := t.Context()
			hfc := getHFC(tc.SpecUpdates)
			r := getTestHFCReconciler(hfc)
			info := rhfcInfo{
				ctx: ctx,
				log: logf.Log.WithName("controllers").WithName("HostFirmwareComponents"),
				hfc: hfc,
			}
			errors := r.validateHostFirmwareComponents(&info)
			if len(errors) == 0 {
				assert.Empty(t, tc.ExpectedErrors[0])
			} else {
				for i := range errors {
					assert.Equal(t, tc.ExpectedErrors[i], errors[i].Error())
				}
			}
		})
	}
}
