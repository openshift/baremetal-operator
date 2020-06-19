// wait-for-ironic waits for the Ironic services to be up and running. It is used during the BMO boostrap to
// ensure the availability of all its required dependencies.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/baremetal/noauth"
	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/drivers"
	noauthintrospection "github.com/gophercloud/gophercloud/openstack/baremetalintrospection/noauth"
	"github.com/gophercloud/gophercloud/pagination"
)

func main() {
	ironicURL := os.Getenv("IRONIC_ENDPOINT")
	ironicMicroversion := os.Getenv("IRONIC_MICROVERSION")
	inspectorURL := os.Getenv("IRONIC_INSPECTOR_ENDPOINT")

	if ironicURL == "" || inspectorURL == "" {
		log.Fatalf("Missing IRONIC_ENDPOINT or IRONIC_INSPECTOR_ENDPOINT env vars")
	}

	if ironicMicroversion == "" {
		ironicMicroversion = "1.52"
	}

	waitForIronicServices(ironicURL, inspectorURL, ironicMicroversion, 60*10)
}

func waitForIronicServices(ironicURL string, inspectorURL string, ironicMicroversion string, timeout int) {

	waitForIronic(ironicURL, ironicMicroversion, timeout)
	waitForInspector(inspectorURL, timeout)
}

func waitForIronic(ironicURL string, ironicMicroversion string, timeout int) {
	//Get client
	ironic, err := noauth.NewBareMetalNoAuth(noauth.EndpointOpts{
		IronicEndpoint: ironicURL,
	})
	if err != nil {
		log.Fatalf("Unable to configure Ironic endpoint: %s", err.Error())
	}
	ironic.Microversion = ironicMicroversion

	// Let's poll the API until it's up, or times out.
	duration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	done := make(chan struct{})
	go func() {
		log.Printf("[INFO] Waiting for Ironic API...")
		waitForAPI(ctx, ironic)
		log.Printf("[INFO] API successfully connected, waiting for conductor...")
		waitForConductor(ctx, ironic)
		close(done)
	}()

	// Wait for done or time out
	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			log.Fatalf("Unable to contact API: %s", err.Error())
		}
	case <-done:
	}
}

func waitForInspector(inspectorURL string, timeout int) {
	//Get client
	inspector, err := noauthintrospection.NewBareMetalIntrospectionNoAuth(noauthintrospection.EndpointOpts{
		IronicInspectorEndpoint: inspectorURL,
	})
	if err != nil {
		log.Fatalf("Unable to configure Inspector endpoint: %s", err.Error())
	}

	// Let's poll the API until it's up, or times out.
	duration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	done := make(chan struct{})
	go func() {
		log.Printf("[INFO] Waiting for Inspector API...")
		waitForAPI(ctx, inspector)
		close(done)
	}()

	// Wait for done or time out
	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			log.Fatalf("Unable to contact Inspector: %s", err)
		}
	case <-done:
	}

}

// Retries an API forever until it responds.
func waitForAPI(ctx context.Context, client *gophercloud.ServiceClient) {
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
			return
		default:
			log.Printf("[DEBUG] Waiting for API to become available...")

			r, err := httpClient.Get(endpoint)
			if err == nil {
				statusCode := r.StatusCode
				r.Body.Close()
				if statusCode == http.StatusOK {
					return
				}
			}

			time.Sleep(5 * time.Second)
		}
	}
}

// Ironic conductor can be considered up when the driver count returns non-zero.
func waitForConductor(ctx context.Context, client *gophercloud.ServiceClient) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Printf("[DEBUG] Waiting for conductor API to become available...")
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
				return
			}

			time.Sleep(5 * time.Second)
		}
	}
}
