package bitcoinclient_test

import (
	"net/http"
	"testing"

	bitcoinclient "github.com/christianb93/bitcoin-controller/internal/bitcoinclient"
)

// TestConfigCreation tests the creation of a configuration
func TestConfigCreation(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	if config.RPCUser != "user" {
		t.Errorf("RPCUser: got %s, expected %s\n", config.RPCUser, "user")
	}
}

// TestClientFromConfig tests the creation of a client from a config
func TestClientFromConfig(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	if client == nil {
		t.Error("Could not create client")
	}
}

// TestNewClien tests the creation of a client
func TestNewClient(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &http.Client{}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
}

/
