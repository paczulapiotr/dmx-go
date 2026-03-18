package models

// DMXMessage is the payload consumed from the RabbitMQ queue.
// It instructs the controller to apply an action (or effect) to a fixture
// starting at a given DMX address.
type DMXMessage struct {
	// StartAddress is the 1-based DMX channel number of the first channel owned by the fixture.
	StartAddress int `json:"start_address"`
	// LightType identifies the fixture model (e.g. "pst10", "b40").
	LightType string `json:"light_type"`
	// Action names the desired colour or effect (e.g. "red", "green", "rainbow", "off").
	Action string `json:"action"`
}
