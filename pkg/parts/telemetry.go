package parts

import (
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"shadow-racer/autopilot/v1/pkg/eventbus"
	"shadow-racer/autopilot/v1/pkg/metrics"
	"shadow-racer/autopilot/v1/pkg/telemetry"
)

type (
	// Telemetry encapsulates the MQTT client in order to send data
	Telemetry struct {
		queue string
		cl    mqtt.Client
	}
)

// NewTelemetry returns a new instance of a telemetry component
func NewTelemetry(broker, queue string) *Telemetry {
	// setup and configuration
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID("telemetry") // FIXME unique id per obu
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	// create a client
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &Telemetry{
		queue: queue,
		cl:    c,
	}
}

// Initialize prepares the telemetry component
func (t *Telemetry) Initialize() error {
	metrics.NewMeter(mTelemetrySend)
	go t.sendData()

	return nil
}

// Reset re-initializes the telemetry component
func (t *Telemetry) Reset() error {
	return nil
}

// Shutdown releases all resources
func (t *Telemetry) Shutdown() error {
	t.cl.Disconnect(250)
	return nil
}

func (t *Telemetry) sendData() {
	ch := eventbus.InstanceOf().Subscribe(topicTelemetrySend)
	for {
		evt := <-ch
		df := evt.Data.(*telemetry.DataFrame)

		payload, err := json.Marshal(df)
		if err == nil {
			t.cl.Publish(t.queue, 0, false, payload)
			metrics.Mark(mTelemetrySend)
		} else {
			logger.Error("Error marshalling data", "err", err.Error())
		}
	}
}
