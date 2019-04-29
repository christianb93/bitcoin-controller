package bitcoinclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

// Error codes. see https://github.com/bitcoin/bitcoin/blob/master/src/rpc/protocol.h
const (
	ErrorCodeNodeAlreadyAdded = -23
	ErrorCodeNodeNotAdded     = -24
)

// RPCError is an error returned by the bitcoind which implements the
// error interface
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e RPCError) Error() string {
	return e.Message
}

// IsNodeAlreadyAdded returns true if the error is an RPCError
// indicating that an instance already exists
func IsNodeAlreadyAdded(err error) bool {
	rpcError, ok := err.(RPCError)
	if !ok {
		return false
	}
	if rpcError.Code != ErrorCodeNodeAlreadyAdded {
		return false
	}
	return true
}

// IsNodeNotAdded returns true if the error is an RPCError
// indicating that a node has not been added before
func IsNodeNotAdded(err error) bool {
	rpcError, ok := err.(RPCError)
	if !ok {
		return false
	}
	if rpcError.Code != ErrorCodeNodeNotAdded {
		return false
	}
	return true
}

// Config represents the configuration information that we need to
// connect to a bitcoind
type Config struct {
	ServerIP    string
	ServerPort  int
	RPCUser     string
	RPCPassword string
}

// RPCRequest is an RPC  request to the bitcoin daemon
type RPCRequest struct {
	JSONRPC int      `json:"jsonrpc"`
	ID      string   `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

// RPCResponse holds a response from the RPC client
type RPCResponse struct {
	Result interface{} `json:"result"`
	Error  RPCError    `json:"error"`
	ID     string      `json:"id"`
}

// AddedNode is a node as returned by the getaddednodeinfo RPC call
type AddedNode struct {
	NodeIP    string `json:"addednode"`
	Connected bool   `json:"connected"`
}

// NewConfig creates a new config
func NewConfig(serverIP string, serverPort int, rpcuser string, rpcpassword string) *Config {
	return &Config{ServerIP: serverIP,
		ServerPort:  serverPort,
		RPCUser:     rpcuser,
		RPCPassword: rpcpassword}
}

// HTTPClient is something that has a Do method. It is of course
// implemented by http.Client but we use an interface to allow for
// unit testing
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// BitcoinClient represents a client
type BitcoinClient struct {
	ClientConfig Config
	HTTPClient   HTTPClient
}

// NewClientForConfig creates a client for a given config
func NewClientForConfig(config *Config) *BitcoinClient {
	return &BitcoinClient{
		ClientConfig: *config,
		HTTPClient:   &http.Client{},
	}
}

// NewClient creates a client for a given config and a given HTTP client
func NewClient(config *Config, client HTTPClient) *BitcoinClient {
	return &BitcoinClient{
		ClientConfig: *config,
		HTTPClient:   client,
	}
}

// RawRequest submits a raw request. If targetIP is empty, the IP from the
// configuration is taken
func (c *BitcoinClient) RawRequest(method string, params []string, targetIP string) (interface{}, error) {
	var rpcResponse RPCResponse
	var actualip = targetIP
	if targetIP == "" {
		actualip = c.ClientConfig.ServerIP
	}
	url := "http://" + actualip + ":" + strconv.Itoa(c.ClientConfig.ServerPort)
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	rpcRequest := RPCRequest{
		JSONRPC: 1,
		ID:      "0",
		Method:  method,
		Params:  params,
	}
	err := encoder.Encode(rpcRequest)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(c.ClientConfig.RPCUser, c.ClientConfig.RPCPassword)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	// now try to decode the body
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&rpcResponse)
	if err != nil {
		return nil, err
	}
	if rpcResponse.Error.Code != 0 {
		return nil, rpcResponse.Error
	}
	return rpcResponse.Result, nil
}

// AddNode invokes the addnode RPC method. If targetIP is the empty
// string, the IP in the config will be used
func (c *BitcoinClient) AddNode(nodeIP string, targetIP string) error {
	_, err := c.RawRequest("addnode", []string{nodeIP, "add"}, targetIP)
	return err
}

// RemoveNode invokes the addnode RPC method. If targetIP is the empty
// string, the IP in the config will be used
func (c *BitcoinClient) RemoveNode(nodeIP string, targetIP string) error {
	_, err := c.RawRequest("addnode", []string{nodeIP, "remove"}, targetIP)
	return err
}

// GetAddedNodes gets the list of added nodes. If targetIP is the empty string,
// the IP in the config will be used
func (c *BitcoinClient) GetAddedNodes(targetIP string) ([]AddedNode, error) {
	rawNodes, err := c.RawRequest("getaddednodeinfo", nil, targetIP)
	if err != nil {
		return nil, err
	}
	// We use the JSON marshaller to convert this into
	// the target structure
	addedNodeList := []AddedNode{}
	bytes, err := json.Marshal(rawNodes)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &addedNodeList)
	if err != nil {
		return nil, err
	}
	return addedNodeList, err
}
