package models

const (
	TypeSimpleUtterance = "SimpleUtterance"
)

type Request struct {
	Request  SimpleUtterance `json:"request"`
	Timezone string          `json:"timezone"`
	Session  Session         `json:"session"`
	Version  string          `json:"version"`
}

type Session struct {
	New bool `json:"new"`
}

type SimpleUtterance struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type Response struct {
	Response ResponsePayload `json:"response"`
	Version  string          `json:"version"`
}

type ResponsePayload struct {
	Text string `json:"text"`
}
