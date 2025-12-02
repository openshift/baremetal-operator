package ironic

import (
	"net/url"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/baremetal/v1/nodes"
	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/baremetal-operator/pkg/hardwareutils/bmc"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner/ironic/clients"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner/ironic/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type BIOSTestBMC struct{}

func (r *BIOSTestBMC) Type() string                                          { return "bios-test" }
func (r *BIOSTestBMC) NeedsMAC() bool                                        { return false }
func (r *BIOSTestBMC) Driver() string                                        { return "bios-test" }
func (r *BIOSTestBMC) DisableCertificateVerification() bool                  { return false }
func (r *BIOSTestBMC) DriverInfo(bmc.Credentials) (i map[string]interface{}) { return }
func (r *BIOSTestBMC) SupportsISOPreprovisioningImage() bool                 { return false }
func (r *BIOSTestBMC) BIOSInterface() string                                 { return "" }
func (r *BIOSTestBMC) BootInterface() string                                 { return "" }
func (r *BIOSTestBMC) FirmwareInterface() string                             { return "" }
func (r *BIOSTestBMC) ManagementInterface() string                           { return "" }
func (r *BIOSTestBMC) PowerInterface() string                                { return "" }
func (r *BIOSTestBMC) RAIDInterface() string                                 { return "" }
func (r *BIOSTestBMC) VendorInterface() string                               { return "" }
func (r *BIOSTestBMC) SupportsSecureBoot() bool                              { return false }
func (r *BIOSTestBMC) RequiresProvisioningNetwork() bool                     { return true }
func (r *BIOSTestBMC) BuildBIOSSettings(_ *bmc.FirmwareConfig) ([]map[string]string, error) {
	return nil, nil
}

func TestService(t *testing.T) {
	bmc.RegisterFactory("bios-test", func(u *url.URL, dcv bool) (bmc.AccessDetails, error) {
		return &BIOSTestBMC{}, nil
	}, []string{})

	nodeUUID := "33ce8659-7400-4c68-9535-d10766f07a58"
	cases := []struct {
		name                 string
		ironic               *testserver.IronicMock
		unprepared           bool
		skipConfig           bool
		matchingSpecStatus   bool // For accept current state abort tests - spec matches status
		expectedStarted      bool
		expectedDirty        bool
		expectedError        bool
		expectedRequestAfter int
	}{
		{
			name: "active, no new steps",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Active),
				UUID:           nodeUUID,
			}),
			skipConfig:           true,
			unprepared:           true,
			expectedStarted:      true,
			expectedRequestAfter: 0,
			expectedDirty:        false,
		},
		{
			name: "active with steps",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Active),
				UUID:           nodeUUID,
			}),
			unprepared:           true,
			expectedStarted:      true,
			expectedRequestAfter: 10,
			expectedDirty:        true,
		},
		{
			name: "serviceFail state(cleaned provision settings)",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
			}),
			expectedStarted:      false,
			expectedRequestAfter: 0,
			expectedDirty:        false,
		},
		{
			name: "serviceFail state(retry)",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
			}),
			unprepared:           true,
			expectedStarted:      true,
			expectedRequestAfter: 10,
			expectedDirty:        true,
		},
		{
			name: "serviceFail state(retry with maintenance)",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
				Maintenance:    true,
			}).NodeMaintenance(nodes.Node{
				UUID: nodeUUID,
			}, false),
			unprepared:      true,
			expectedStarted: false,
			expectedDirty:   true,
		},
		{
			name: "servicing state",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Servicing),
				UUID:           nodeUUID,
			}),
			expectedStarted:      false,
			expectedRequestAfter: 10,
			expectedDirty:        true,
		},
		{
			name: "serviceWait state",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceWait),
				UUID:           nodeUUID,
			}),
			expectedStarted:      false,
			expectedRequestAfter: 10,
			expectedDirty:        true,
		},
		{
			name: "active state(servicing finished)",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Active),
				UUID:           nodeUUID,
			}),
			expectedStarted:      false,
			expectedRequestAfter: 0,
			expectedDirty:        false,
		},
		{
			name: "unexpected state",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Cleaning),
				UUID:           nodeUUID,
			}),
			expectedStarted:      false,
			expectedRequestAfter: 0,
			expectedDirty:        false,
			expectedError:        true,
		},
		// Service abort tests - Complete spec removal triggers abort
		{
			name: "serviceFail_abort_no_changes_detected",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
			}),
			skipConfig:           true, // No firmware specs = ChangeDetected false for both
			expectedStarted:      true, // Abort triggers state change
			expectedRequestAfter: 10,   // Abort returns RequeueAfter: 10s
			expectedDirty:        true, // Abort returns Dirty: true
		},
		{
			name: "servicing_continue_on_completion",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Servicing),
				UUID:           nodeUUID,
			}),
			skipConfig:           true,  // No firmware specs = ChangeDetected false for both
			expectedStarted:      false, // No abort - continues servicing
			expectedRequestAfter: 10,    // Continues with normal requeue delay
			expectedDirty:        true,  // Operation continues
		},
		{
			name: "serviceWait_continue_on_completion",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceWait),
				UUID:           nodeUUID,
			}),
			skipConfig:           true,  // No firmware specs = ChangeDetected false for both
			expectedStarted:      false, // No abort - continues servicing
			expectedRequestAfter: 10,    // Continues with normal requeue delay
			expectedDirty:        true,  // Operation continues
		},
		// Service abort tests - No firmware changes detected (both HFS and HFC ChangeDetected=false)
		{
			name: "serviceFail_abort_no_changes_spec_matches_status",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
			}),
			matchingSpecStatus:   true, // ChangeDetected=false for both HFS and HFC
			expectedStarted:      true, // Abort triggers state change
			expectedRequestAfter: 10,   // Abort returns RequeueAfter: 10s
			expectedDirty:        true, // Abort returns Dirty: true
		},
		{
			name: "servicing_continue_when_spec_matches_status",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.Servicing),
				UUID:           nodeUUID,
			}),
			matchingSpecStatus:   true,  // ChangeDetected=false for both HFS and HFC
			expectedStarted:      false, // No abort - continues servicing
			expectedRequestAfter: 10,    // Continues with normal requeue delay
			expectedDirty:        true,  // Operation continues
		},
		{
			name: "serviceWait_continue_when_spec_matches_status",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceWait),
				UUID:           nodeUUID,
			}),
			matchingSpecStatus:   true,  // ChangeDetected=false for both HFS and HFC
			expectedStarted:      false, // No abort - continues servicing
			expectedRequestAfter: 10,    // Continues with normal requeue delay
			expectedDirty:        true,  // Operation continues
		},
		// Mixed scenarios - when only one spec matches status (should NOT abort)
		{
			name: "serviceFail_mixed_hfs_matches_hfc_differs",
			ironic: testserver.NewIronic(t).WithDefaultResponses().Node(nodes.Node{
				ProvisionState: string(nodes.ServiceFail),
				UUID:           nodeUUID,
			}),
			unprepared:           true, // Retry servicing case
			expectedStarted:      true, // Should restart servicing, not abort
			expectedRequestAfter: 10,   // Normal servicing retry
			expectedDirty:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ironic != nil {
				tc.ironic.Start()
				defer tc.ironic.Stop()
			}

			host := makeHost()
			host.Status.Provisioning.ID = nodeUUID
			prepData := provisioner.ServicingData{}
			if !tc.skipConfig {
				host.Spec.BMC.Address = "bios-test://test.bmc/"
				// Handle mixed scenario test case
				if tc.name == "serviceFail_mixed_hfs_matches_hfc_differs" {
					// HFS has no changes (ChangeDetected=false) but HFC has changes (ChangeDetected=true)
					prepData.FirmwareSettingsChangeDetected = false  // HFS matches status
					prepData.FirmwareComponentsChangeDetected = true // HFC differs from status
					// HFS settings match (no service steps from settings)
					prepData.ActualFirmwareSettings = metal3api.SettingsMap{
						"ProcVirtualization": "Enabled", // Same as spec
					}
					prepData.TargetFirmwareSettings = metal3api.DesiredSettingsMap{
						"ProcVirtualization": intstr.FromString("Enabled"), // Same as actual
					}
					// HFC components differ (should generate service steps)
					prepData.TargetFirmwareComponents = []metal3api.FirmwareUpdate{
						{
							Component: "bios",
							URL:       "https://example.com/bios-v1.2.3.bin",
						},
					}
				} else if tc.matchingSpecStatus {
					// No changes detected: ChangeDetected=false for both HFS and HFC triggers abort
					prepData.FirmwareSettingsChangeDetected = false   // No changes detected, should abort
					prepData.FirmwareComponentsChangeDetected = false // No changes detected, should abort
					// Provide matching firmware settings (spec matches status)
					prepData.ActualFirmwareSettings = metal3api.SettingsMap{
						"ProcVirtualization": "Enabled",
					}
					prepData.TargetFirmwareSettings = metal3api.DesiredSettingsMap{
						"ProcVirtualization": intstr.FromString("Enabled"), // Same as actual
					}
				} else {
					// Normal case: ChangeDetected=true (changes pending, no abort)
					prepData.FirmwareSettingsChangeDetected = true    // Changes pending, continue servicing
					prepData.FirmwareComponentsChangeDetected = false // No components spec for these tests
					// Provide actual firmware settings that differ to generate service steps
					prepData.ActualFirmwareSettings = metal3api.SettingsMap{
						"ProcVirtualization": "Disabled",
					}
					prepData.TargetFirmwareSettings = metal3api.DesiredSettingsMap{
						"ProcVirtualization": intstr.FromString("Enabled"), // Different from actual
					}
				}
			} else {
				// For skipConfig cases (no firmware specs), ChangeDetected is false for both
				prepData.FirmwareSettingsChangeDetected = false
				prepData.FirmwareComponentsChangeDetected = false
			}

			publisher := func(reason, message string) {}
			auth := clients.AuthConfig{Type: clients.NoAuth}
			prov, err := newProvisionerWithSettings(host, bmc.Credentials{}, publisher, tc.ironic.Endpoint(), auth)
			if err != nil {
				t.Fatalf("could not create provisioner: %s", err)
			}
			prov.availableFeatures = clients.AvailableFeatures{MaxVersion: 87}

			result, started, err := prov.Service(prepData, tc.unprepared, tc.unprepared)

			assert.Equal(t, tc.expectedStarted, started)
			assert.Equal(t, tc.expectedDirty, result.Dirty)
			assert.Equal(t, time.Second*time.Duration(tc.expectedRequestAfter), result.RequeueAfter)
			if !tc.expectedError {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
