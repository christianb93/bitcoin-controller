package main

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

// TestRawRequestIntegration tests a raw request for the getnetworkinfo method
// i.e. without params
func TestRawRequestIntegration(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	resp, err := client.RawRequest("getnetworkinfo", nil, nil)
	if err != nil {
		t.Errorf("Unexpected error %s\n", err)
	}
	if resp == nil {
		t.Error("Response should not be nil")
	}
	_, ok := resp.(map[string]interface{})
	if !ok {
		t.Errorf("Unexpected type %T of response\n", resp)
	}
}

// TestRawRequestMethodUnknownIntegration tests a raw request for
// an unknown method
func TestRawRequestMethodUnknownIntegration(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	_, err := client.RawRequest("getnetworkinfox", nil, nil)
	if err == nil {
		t.Error("Expected error")
	}
}

// TestAddNodeIntegration tests that a node can be added
func TestAddNodeIntegration(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.2", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	// Now overwrite the config - this will also test that we use
	// this config and not the client config
	config = bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	err := client.AddNode("127.0.0.3", config)
	if err != nil {
		t.Errorf("Unexpected error %s\n", err)
	}
	// Verify that node is in the list of added nodes
	addedNodeList, err := client.GetAddedNodes(config)
	if err != nil {
		t.Errorf("Have unexpected error %s\n", err)
		t.FailNow()
	}
	var found = true
	for _, addedNode := range addedNodeList {
		if addedNode.NodeIP == "127.0.0.3" {
			found = true
		}
	}
	if !found {
		t.Errorf("Did not find expected node\n")
		t.FailNow()
	}
	// Now remove node again
	err = client.RemoveNode("127.0.0.3", config)
	if err != nil {
		t.Errorf("Unexpected error %s\n", err)
	}
	// and make sure that is has been removed
	addedNodeList, err = client.GetAddedNodes(config)
	if err != nil {
		t.Errorf("Have unexpected error %s\n", err)
		t.FailNow()
	}
	found = false
	for _, addedNode := range addedNodeList {
		if addedNode.NodeIP == "127.0.0.3" {
			found = true
		}
	}
	if found {
		t.Errorf("Node still in list\n")
		t.FailNow()
	}
}

// TestAddNodeTwiceIntegration tests that a node cannot be
// added twice
func TestAddNodeTwiceIntegration(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	err := client.AddNode("127.0.0.3", nil)
	if err != nil {
		t.Errorf("Unexpected error %s\n", err)
	}
	// try to add it twice
	err = client.AddNode("127.0.0.3", nil)
	if err == nil {
		t.Errorf("Did expect an error but did not get one\n")
	}
	_, ok := err.(bitcoinclient.RPCError)
	if !ok {
		t.Errorf("Did expect RPC error, but got error of type %T\n", err)
	}
	if !bitcoinclient.IsNodeAlreadyAdded(err) {
		t.Error("Error not as expected")
	}
	// Now remove node again
	err = client.RemoveNode("127.0.0.3", nil)
	if err != nil {
		t.Errorf("Unexpected error %s\n", err)
	}
}
