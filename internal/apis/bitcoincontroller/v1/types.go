package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BitcoinNetwork specifies a bitcoin test network
type BitcoinNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BitcoinNetworkSpec   `json:"spec"`
	Status BitcoinNetworkStatus `json:"status"`
}

// +k8s:deepcopy-gen=true

// BitcoinNetworkSpec is the to-be state of a bitcoin network
type BitcoinNetworkSpec struct {
	Nodes int32 `json:"nodes"`
}

// +k8s:deepcopy-gen=true

// BitcoinNetworkNode represents an individual node of the bitcoin network (i.e. pod running
// the bitcoind container
type BitcoinNetworkNode struct {
	// a number from 0...n-1 in a deployment with n nodes, corresponding to
	// the ordinal in the stateful set
	Ordinal int32 `json:"ordinal"`
	// is this node ready, i.e. is the bitcoind RPC server ready to accept requests?
	Ready bool `json:"ready"`
	// the IP of the node, i.e. the IP of the pod running the node
	IP string `json:"ip"`
	// the name of the node
	NodeName string `json:"nodeName"`
	// the DNS name
	DNSName string `json:"dnsName"`
}

// +k8s:deepcopy-gen=true

// BitcoinNetworkStatus is the as-is state of a bitcoin network
type BitcoinNetworkStatus struct {
	Nodes []BitcoinNetworkNode `json:"nodes"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BitcoinNetworkList is a list of bitcoin networks
type BitcoinNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BitcoinNetwork `json:"items"`
}
