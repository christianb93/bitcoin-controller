package bitcoinclient_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	bitcoinclient "github.com/christianb93/bitcoin-controller/internal/bitcoinclient"
)

// A fake http client
type fakeHttpClient struct {
	AddedNodeList []bitcoinclient.AddedNode
}

func (f *fakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	// Try to extract the method from the request
	// Decode this into a RPCRequest
	rpcRequest := bitcoinclient.RPCRequest{}
	json.NewDecoder(req.Body).Decode(&rpcRequest)
	// Prepare a default response string
	response := &bitcoinclient.RPCResponse{
		Result: nil,
		Error: bitcoinclient.RPCError{
			Code:    -32601,
			Message: "Method unknown",
		},
		ID: rpcRequest.ID,
	}
	switch rpcRequest.Method {
	case "getpeerinfo":
		response.Error = bitcoinclient.RPCError{}
		response.Result = nil
	case "getaddednodeinfo":
		response.Error = bitcoinclient.RPCError{}
		response.Result = f.AddedNodeList
	case "addnode":
		// First argument is node, second argument is add/remove
		nodeIP := rpcRequest.Params[0]
		switch rpcRequest.Params[1] {
		case "add":
			response.Error = bitcoinclient.RPCError{}
			f.AddedNodeList = append(f.AddedNodeList, bitcoinclient.AddedNode{
				NodeIP:    nodeIP,
				Connected: true,
			})
		case "remove":
			response.Error = bitcoinclient.RPCError{}
			newNodeList := make([]bitcoinclient.AddedNode, 0)
			for _, node := range f.AddedNodeList {
				if node.NodeIP != nodeIP {
					newNodeList = append(newNodeList, node)
				}
			}
			f.AddedNodeList = newNodeList
		}
	}
	v, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal response, error is %s\n", err)
	}
	resp := &http.Response{
		Body: ioutil.NopCloser(bytes.NewReader(v)),
	}
	return resp, nil
}

// TestConfigCreationUnit tests the creation of a configuration
func TestConfigCreationUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	if config.RPCUser != "user" {
		t.Errorf("RPCUser: got %s, expected %s\n", config.RPCUser, "user")
	}
}

// TestClientFromConfigUnit tests the creation of a client from a config
func TestClientFromConfigUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	client := bitcoinclient.NewClientForConfig(config)
	if client == nil {
		t.Error("Could not create client")
	}
}

// TestNewClientUnit tests the creation of a client
func TestNewClientUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &http.Client{}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
}

// Test a raw request
func TestRawRequestUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &fakeHttpClient{
		AddedNodeList: []bitcoinclient.AddedNode{
			bitcoinclient.AddedNode{
				NodeIP:    "10.0.0.1",
				Connected: true,
			},
		},
	}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
	// Run a raw request
	_, err := client.RawRequest("getpeerinfo", []string{}, config)
	if err != nil {
		t.Errorf("Got error %s\n", err)
	}
}

// Test GetAddedNodes
func TestGetAddedNodesUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &fakeHttpClient{
		AddedNodeList: []bitcoinclient.AddedNode{
			bitcoinclient.AddedNode{
				NodeIP:    "10.0.0.3",
				Connected: true,
			},
		},
	}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
	// Run request
	addedNodes, err := client.GetAddedNodes(config)
	if err != nil {
		t.Errorf("Got error %s\n", err)
	}
	if !reflect.DeepEqual(addedNodes, httpClient.AddedNodeList) {
		t.Errorf("Equal node lists are not equal\n")
	}
}

// Test AddNode
func TestGetAddNodeUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &fakeHttpClient{
		AddedNodeList: []bitcoinclient.AddedNode{},
	}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
	// Run request
	err := client.AddNode("10.0.0.5", config)
	if err != nil {
		t.Errorf("Got error %s\n", err)
	}
	// Check that node has been added
	if len(httpClient.AddedNodeList) != 1 {
		t.Errorf("Have wrong number of items in added node list\n")
	}
}

// Test RemoveNode
func TestGetRemoveNodeUnit(t *testing.T) {
	config := bitcoinclient.NewConfig("127.0.0.1", 18332, "user", "password")
	httpClient := &fakeHttpClient{
		AddedNodeList: []bitcoinclient.AddedNode{
			bitcoinclient.AddedNode{
				NodeIP:    "10.0.0.3",
				Connected: true,
			},
		},
	}
	client := bitcoinclient.NewClient(config, httpClient)
	if client == nil {
		t.Error("Could not create client")
	}
	// Run request
	err := client.RemoveNode("10.0.0.3", config)
	if err != nil {
		t.Errorf("Got error %s\n", err)
	}
	// Check that node has been added
	if len(httpClient.AddedNodeList) != 0 {
		t.Errorf("Have wrong number of items in added node list\n")
	}
}
