package main

import "time"

// Msg is a raw message
type Msg struct {
	Stamp   time.Time `json:"timestamp"`
	MsgType string    `json:"type"`
	Msg     []byte    `json:"msg"`
}

// HostMsg is a HostState, HostAlert, HostFlappingAlert or HostFlappingAlert
type HostMsg struct {
	SubType    string `json:"type"`
	HostName   string `json:"hostname"`
	State      string `json:"state"`
	StateType  string `json:"state_type"`
	StateCount int64  `json:"state_count"`
	Message    string `json:"message"`
}

// ServiceMsg is a ServiceState, ServiceAlert, ServiceDownAlert or ServiceFlappingAlert
type ServiceMsg struct {
	SubType     string `json:"type"`
	HostName    string `json:"hostname"`
	ServiceName string `json:"service_name"`
	State       string `json:"state"`
	StateType   string `json:"state_type"`
	StateCount  int64  `json:"state_count"`
	Message     string `json:"message"`
}
