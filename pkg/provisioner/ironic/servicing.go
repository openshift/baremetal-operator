package ironic

import (
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/baremetal/v1/nodes"
	"github.com/metal3-io/baremetal-operator/pkg/hardwareutils/bmc"
	"github.com/metal3-io/baremetal-operator/pkg/provisioner"
)

func (p *ironicProvisioner) buildServiceSteps(bmcAccess bmc.AccessDetails, data provisioner.ServicingData) (serviceSteps []nodes.ServiceStep, err error) {
	// Get the subset (currently 3) of vendor specific BIOS settings converted from common names
	var firmwareConfig *bmc.FirmwareConfig
	if data.FirmwareConfig != nil {
		bmcConfig := bmc.FirmwareConfig(*data.FirmwareConfig)
		firmwareConfig = &bmcConfig
	}
	fwConfigSettings, err := bmcAccess.BuildBIOSSettings(firmwareConfig)
	if err != nil {
		return nil, err
	}

	newSettings := p.getNewFirmwareSettings(data.ActualFirmwareSettings, data.TargetFirmwareSettings, fwConfigSettings)
	if len(newSettings) != 0 {
		p.log.Info("Applying BIOS config clean steps", "settings", newSettings)
		serviceSteps = append(
			serviceSteps,
			nodes.ServiceStep{
				Interface: nodes.InterfaceBIOS,
				Step:      "apply_configuration",
				Args: map[string]interface{}{
					"settings": newSettings,
				},
			},
		)
	}

	newUpdates := p.getFirmwareComponentsUpdates(data.TargetFirmwareComponents)
	if len(newUpdates) != 0 {
		p.log.Info("Applying Firmware Update clean steps", "settings", newUpdates)
		serviceSteps = append(
			serviceSteps,
			nodes.ServiceStep{
				Interface: nodes.InterfaceFirmware,
				Step:      "update",
				Args: map[string]interface{}{
					"settings": newUpdates,
				},
			},
		)
	}

	return serviceSteps, nil
}

func (p *ironicProvisioner) startServicing(bmcAccess bmc.AccessDetails, ironicNode *nodes.Node, data provisioner.ServicingData) (success bool, result provisioner.Result, err error) {
	// Build service steps
	serviceSteps, err := p.buildServiceSteps(bmcAccess, data)
	if err != nil {
		result, err = operationFailed(err.Error())
		return
	}

	// Start servicing
	if len(serviceSteps) != 0 {
		p.log.Info("remove existing configuration and set new configuration", "serviceSteps", serviceSteps)
		return p.tryChangeNodeProvisionState(
			ironicNode,
			nodes.ProvisionStateOpts{
				Target:       nodes.TargetService,
				ServiceSteps: serviceSteps,
			},
		)
	}
	result, err = operationComplete()
	return
}

func (p *ironicProvisioner) abortServicing(ironicNode *nodes.Node) (result provisioner.Result, started bool, err error) {
	// Clear maintenance flag first if it's set
	if ironicNode.Maintenance {
		p.log.Info("clearing maintenance flag before aborting servicing")
		result, err = p.setMaintenanceFlag(ironicNode, false, "")
		return result, started, err
	}

	// Set started to let the controller know about the change
	p.log.Info("aborting servicing due to removal of spec.updates/spec.settings")
	started, result, err = p.tryChangeNodeProvisionState(
		ironicNode,
		nodes.ProvisionStateOpts{Target: nodes.TargetAbort},
	)
	p.log.Info("janders_debug: abort result", "started", started, "result", result, "error", err)
	return
}

func (p *ironicProvisioner) shouldAbortServicing(data provisioner.ServicingData) bool {
	// Determine if user cleared the appropriate specs to trigger abort
	// Logic:
	// - If only settings triggered servicing, abort only if settings spec is cleared
	// - If only components triggered servicing, abort only if components spec is cleared
	// - If both triggered servicing, abort only if both specs are cleared
	if data.ServicingTriggeredBySettings && data.ServicingTriggeredByComponents {
		// Both triggered - must clear both to abort
		return !data.HasFirmwareSettingsSpec && !data.HasFirmwareComponentsSpec
	} else if data.ServicingTriggeredBySettings {
		// Only settings triggered - must clear settings to abort
		return !data.HasFirmwareSettingsSpec
	} else if data.ServicingTriggeredByComponents {
		// Only components triggered - must clear components to abort
		return !data.HasFirmwareComponentsSpec
	}

	// Neither triggered (shouldn't happen during servicing) - don't abort
	return false
}

func (p *ironicProvisioner) Service(data provisioner.ServicingData, unprepared, restartOnFailure bool) (result provisioner.Result, started bool, err error) {
	if !p.availableFeatures.HasServicing() {
		result, err = operationFailed(fmt.Sprintf("servicing not supported: requires API version 1.87, available is 1.%d", p.availableFeatures.MaxVersion))
		return result, started, err
	}

	bmcAccess, err := p.bmcAccess()
	if err != nil {
		result, err = transientError(err)
		return result, started, err
	}

	ironicNode, err := p.getNode()
	if err != nil {
		result, err = transientError(err)
		return result, started, err
	}

	// Check if there are any pending updates
	serviceSteps, err := p.buildServiceSteps(bmcAccess, data)
	if err != nil {
		result, err = operationFailed(err.Error())
		return result, started, err
	}

	p.log.Info("janders_debug: servicing state check",
		"hasSettingsSpec", data.HasFirmwareSettingsSpec,
		"hasComponentsSpec", data.HasFirmwareComponentsSpec,
		"triggeredBySettings", data.ServicingTriggeredBySettings,
		"triggeredByComponents", data.ServicingTriggeredByComponents,
		"serviceStepsCount", len(serviceSteps),
		"nodeState", ironicNode.ProvisionState)

	switch nodes.ProvisionState(ironicNode.ProvisionState) {
	case nodes.ServiceFail:
		// When servicing failed and user actually removed the appropriate specs,
		// we need to abort the servicing operation to back out
		if p.shouldAbortServicing(data) {
			p.log.Info("aborting servicing because user cleared the relevant spec.updates/spec.settings")
			return p.abortServicing(ironicNode)
		}

		// When servicing failed and there are pending updates, we need to clean host provisioning settings
		// If restartOnFailure is false, it means the settings aren't cleared.
		if !restartOnFailure {
			result, err = operationFailed(ironicNode.LastError)
			return result, started, err
		}

		if ironicNode.Maintenance {
			p.log.Info("clearing maintenance flag after a servicing failure")
			result, err = p.setMaintenanceFlag(ironicNode, false, "")
			return result, started, err
		}

		p.log.Info("restarting servicing because of a previous failure")
		unprepared = true
		fallthrough
	case nodes.Active:
		if unprepared {
			started, result, err = p.startServicing(bmcAccess, ironicNode, data)
			if started || result.Dirty || result.ErrorMessage != "" || err != nil {
				return result, started, err
			}
			// nothing to do
			started = true
		}
		// Servicing finished
		p.log.Info("servicing finished on the host")
		result, err = operationComplete()
	case nodes.Servicing, nodes.ServiceWait:
		// If user cleared the relevant spec.updates/spec.settings while servicing is in progress, abort immediately
		if p.shouldAbortServicing(data) {
			p.log.Info("aborting in-progress servicing because user cleared the relevant spec.updates/spec.settings")
			return p.abortServicing(ironicNode)
		}

		p.log.Info("waiting for host to become active",
			"state", ironicNode.ProvisionState,
			"serviceStep", ironicNode.ServiceStep)
		result, err = operationContinuing(provisionRequeueDelay)

	default:
		result, err = transientError(fmt.Errorf("have unexpected ironic node state %s", ironicNode.ProvisionState))
	}
	return result, started, err
}
