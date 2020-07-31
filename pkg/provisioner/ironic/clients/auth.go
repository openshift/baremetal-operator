package clients

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

// AuthType is the method of authenticating requests to the server
type AuthType string

const (
	// NoAuth uses no authentication
	NoAuth AuthType = "noauth"
	// HTTPBasicAuth uses HTTP Basic Authentication
	HTTPBasicAuth AuthType = "http_basic"
)

// AuthConfig contains data needed to configure authentication in the client
type AuthConfig struct {
	Type     AuthType
	Username string
	Password string
}

func (auth *AuthConfig) load(clientType string) error {
	authPath := path.Join("/auth", clientType)

	if _, err := os.Stat(authPath); err != nil {
		if os.IsNotExist(err) {
			auth.Type = NoAuth
			return nil
		}
		return err
	}
	auth.Type = HTTPBasicAuth

	username, err := ioutil.ReadFile(filepath.Clean(path.Join(authPath, "username")))
	if err != nil {
		return err
	}
	if len(username) == 0 {
		return fmt.Errorf("Empty HTTP Basic Auth username")
	}
	auth.Username = string(username)

	password, err := ioutil.ReadFile(filepath.Clean(path.Join(authPath, "password")))
	if err != nil {
		return err
	}
	if len(password) == 0 {
		return fmt.Errorf("Empty HTTP Basic Auth password")
	}
	auth.Password = string(password)

	return nil
}

// LoadAuth loads the Ironic and Inspector configuration from the environment
func LoadAuth() (ironicAuth, inspectorAuth AuthConfig, err error) {
	err = ironicAuth.load("ironic")
	if err != nil {
		return
	}
	err = inspectorAuth.load("ironic-inspector")
	return
}

// ConfigFromEndpointURL returns an endpoint and an auth config from an
// endpoint URL that may contain HTTP basic auth credentials.
func ConfigFromEndpointURL(endpointURL string) (endpoint string, auth AuthConfig, err error) {
	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return
	}

	if parsedURL.User != nil {
		var hasPasswd bool
		auth.Type = HTTPBasicAuth
		auth.Username = parsedURL.User.Username()
		auth.Password, hasPasswd = parsedURL.User.Password()
		if !hasPasswd {
			err = fmt.Errorf("No password supplied for HTTP Basic Auth")
		}
		parsedURL.User = nil
	} else {
		auth.Type = NoAuth
	}

	endpoint = parsedURL.String()
	return
}
