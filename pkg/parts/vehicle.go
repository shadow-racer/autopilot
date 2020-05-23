package parts

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/majordomusio/commons/pkg/util"

	"shadow-racer/autopilot/v1/pkg/eventbus"
	"shadow-racer/autopilot/v1/pkg/metrics"
	"shadow-racer/autopilot/v1/pkg/obu"
	"shadow-racer/autopilot/v1/pkg/telemetry"
)

const (
	TICK = 40 // update every TICK

	stateDriving = "DRIVING"
	stateStopped = "STOPPED"
)

type (

	// Vehicle holds state information of a generic vehicle
	Vehicle struct {
		Mode     string  `json:"mode"` // some string, e.g. DRIVING, STOPPED, etc
		Throttle float32 `json:"th"`   // -100 .. 100
		Steering float32 `json:"st"`   // in deg, 0 is straight ahead
		Heading  float32 `json:"head"` // heading of the vehicle 0 -> North, 90 -> East ...
		Batch    int64   `json:"batch"`
		TS       int64   `json:"ts"` // timestamp
	}

	// VehicleState is an aggregate of vehicle state and other state information
	VehicleState struct {
		mutex *sync.Mutex
		obu   obu.OnboardUnit

		Recording bool
		vehicle   *Vehicle
		image     []byte
	}
)

// NewVehicleState creates a new state instance
func NewVehicleState(o obu.OnboardUnit) *VehicleState {
	return &VehicleState{
		mutex: &sync.Mutex{},
		obu:   o,
		vehicle: &Vehicle{
			Mode:     stateStopped,
			Throttle: 0.0,
			Steering: 0.0,
			Heading:  0.0,
			Batch:    0,
			TS:       util.TimestampNano(),
		},
	}
}

// Initialize prepares the device/component
func (v *VehicleState) Initialize() error {
	metrics.NewMeter(mStateUpdate)

	// start the event processing
	go v.RemoteStateHandler()
	go v.RemoteImageHandler()
	go v.PeriodicUpdate()

	return nil
}

// Reset re-initializes the device/component
func (v *VehicleState) Reset() error {
	return nil
}

// Shutdown releases all resources/component
func (v *VehicleState) Shutdown() error {
	return nil
}

// RemoteStateHandler listens remote state changes and updates the vehicle state accordingly
func (v *VehicleState) RemoteStateHandler() {
	logger.Info("Starting the remote state handler", "rxv", topicRCStateReceive, "txv", topicRCStateUpdate)

	ch := eventbus.InstanceOf().Subscribe(topicRCStateReceive)
	for {
		evt := <-ch
		state := evt.Data.(RemoteState)

		v.mutex.Lock()

		if state.Mode != v.vehicle.Mode {
			if state.Mode == stateDriving {
				// assumes v.vehicle.Mode == STOPPED
				//o.TailLights(4000, true) // FIXME enable tail lights
			} else if state.Mode == stateStopped {
				//o.TailLightsOff() // FIXME disable tail lights
				v.vehicle.Throttle = 0.0
				v.vehicle.Steering = 0.0
			} else {
				// FIXME should not happen
			}
			v.vehicle.Mode = state.Mode
		} else {
			//v.vehicle.Steering = 100.0 * ((float32(o.servo.MaxRange) / 90.0) * state.Steering)
			v.vehicle.Steering = 100.0 * ((30.0 / 90.0) * state.Steering) // FIXME -> o.servo.MaxRange config
			v.vehicle.Throttle = 100.0 * state.Throttle
		}

		if state.Recording != v.Recording {
			baseURL := "http://localhost:3001" // FIXME configuration

			if state.Recording == true {
				v.vehicle.Batch = util.Timestamp()
				v.Recording = true

				resp, err := http.Get(fmt.Sprintf("%s/start?ts=%d", baseURL, v.vehicle.Batch))

				if err != nil {
					logger.Error("Error toggling recording", "err", err.Error())
				} else {
					logger.Info("Started recording", "ts", v.vehicle.Batch)
				}
				defer resp.Body.Close()
			} else {
				v.Recording = false
				resp, err := http.Get(baseURL + "/stop")

				if err != nil {
					logger.Error("Error toggling recording", "err", err.Error())
				} else {
					logger.Info("Stopped recording")
				}
				defer resp.Body.Close()
			}
		}

		v.vehicle.TS = util.TimestampNano()

		// publish the new state
		eventbus.InstanceOf().Publish(topicRCStateUpdate, v.vehicle.Clone())

		// set the actuators
		v.obu.Direction(int(v.vehicle.Steering))
		v.obu.Throttle(int(v.vehicle.Throttle))

		v.mutex.Unlock()
	}
}

// RemoteImageHandler receives individual camera frames
func (v *VehicleState) RemoteImageHandler() {
	logger.Info("Starting the remote image handler", "rxv", topicImageReceive)

	ch := eventbus.InstanceOf().Subscribe(topicImageReceive)
	for {
		evt := <-ch

		// just update with the latest image
		v.mutex.Lock()
		v.image = evt.Data.([]byte)
		v.mutex.Unlock()
	}
}

// PeriodicUpdate sends telemetry data in fixed intervals
func (v *VehicleState) PeriodicUpdate() {
	logger.Info("Starting periodic state update", "TICK", TICK)

	// periodic background processes
	ticks := time.NewTicker(time.Millisecond * time.Duration(TICK)).C // about 20x/s
	for {
		<-ticks

		v.mutex.Lock()

		if v.Recording {
			ts := util.TimestampNano()

			// send the state
			df1 := v.vehicle.toDataFrame()
			df1.TS = ts
			eventbus.InstanceOf().Publish(topicTelemetrySend, df1)

			// send the current image
			df2 := telemetry.DataFrame{
				DeviceID: "shadow-racer",
				Batch:    v.vehicle.Batch,
				TS:       ts,
				Type:     telemetry.BLOB,
				Blob:     string(v.image),
			}
			eventbus.InstanceOf().Publish(topicTelemetrySend, &df2)
		}

		v.mutex.Unlock()

		metrics.Mark(mStateUpdate)
	}
}

// Clone returns a deep copy the vehicle state
func (v *Vehicle) Clone() *Vehicle {
	return &Vehicle{
		Mode:     v.Mode,
		Throttle: v.Throttle,
		Steering: v.Steering,
		Heading:  v.Heading,
		Batch:    v.Batch,
		TS:       v.TS,
	}
}

func (v *Vehicle) toDataFrame() *telemetry.DataFrame {
	df := telemetry.DataFrame{
		DeviceID: "shadow-racer",
		Batch:    v.Batch,
		TS:       v.TS,
		Type:     telemetry.KV,
		Data:     make(map[string]string),
	}
	df.Data["mode"] = v.Mode
	df.Data["th"] = fmt.Sprintf("%f", v.Throttle)
	df.Data["st"] = fmt.Sprintf("%f", v.Steering)
	df.Data["head"] = fmt.Sprintf("%f", v.Heading)

	return &df
}