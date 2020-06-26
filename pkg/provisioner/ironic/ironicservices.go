package ironic

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/drivers"
	"github.com/gophercloud/gophercloud/pagination"
)

func waitForIronicServices(client *gophercloud.ServiceClient, inspector *gophercloud.ServiceClient, timeout int) bool {

	return waitForIronic(client, timeout) &&
		waitForInspector(inspector, timeout)
}

func waitForIronic(ironic *gophercloud.ServiceClient, timeout int) bool {
	// Let's poll the API until it's up, or times out.
	duration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	log.Info("Waiting for Ironic services...")
	res := waitForAPI(ctx, ironic) && waitForConductor(ctx, ironic)

	return res
}

func waitForInspector(inspector *gophercloud.ServiceClient, timeout int) bool {
	// Let's poll the API until it's up, or times out.
	duration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	log.Info("Waiting for Inspector API...")
	return waitForAPI(ctx, inspector)
}

// Retries an API forever until it responds.
func waitForAPI(ctx context.Context, client *gophercloud.ServiceClient) bool {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// NOTE: Some versions of Ironic inspector returns 404 for /v1/ but 200 for /v1,
	// which seems to be the default behavior for Flask. Remove the trailing slash
	// from the client endpoint.
	endpoint := strings.TrimSuffix(client.Endpoint, "/")

	for {
		select {
		case <-ctx.Done():
			log.Info("Unable to contact API")
			return false
		default:
			log.Info("Waiting for API to become available...")

			r, err := httpClient.Get(endpoint)
			if err == nil {
				statusCode := r.StatusCode
				r.Body.Close()
				if statusCode == http.StatusOK {
					return true
				}
			}

			time.Sleep(5 * time.Second)
		}
	}
}

// Ironic conductor can be considered up when the driver count returns non-zero.
func waitForConductor(ctx context.Context, client *gophercloud.ServiceClient) bool {
	for {
		select {
		case <-ctx.Done():
			log.Info("Unable to contact API")
			return false
		default:
			log.Info("Waiting for conductor API to become available...")
			driverCount := 0

			drivers.ListDrivers(client, drivers.ListDriversOpts{
				Detail: false,
			}).EachPage(func(page pagination.Page) (bool, error) {
				actual, err := drivers.ExtractDrivers(page)
				if err != nil {
					return false, err
				}
				driverCount += len(actual)
				return true, nil
			})
			// If we have any drivers, conductor is up.
			if driverCount > 0 {
				return true
			}

			time.Sleep(5 * time.Second)
		}
	}
}
