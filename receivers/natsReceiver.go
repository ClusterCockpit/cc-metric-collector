package receivers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
	nats "github.com/nats-io/nats.go"
)

type NatsReceiverConfig struct {
	defaultReceiverConfig
	Addr    string `json:"address"`
	Port    string `json:"port"`
	Subject string `json:"subject"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	NkeyFile string `json:"nkey_file,omitempty"`
}

type NatsReceiver struct {
	receiver
	nc *nats.Conn
	//meta   map[string]string
	config NatsReceiverConfig
}

// Start subscribes to the configured NATS subject
// Messages wil be handled by r._NatsReceive
func (r *NatsReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")
	r.nc.Subscribe(r.config.Subject, r._NatsReceive)
}

// _NatsReceive receives subscribed messages from the NATS server
func (r *NatsReceiver) _NatsReceive(m *nats.Msg) {

	if r.sink != nil {
		d := influx.NewDecoderWithBytes(m.Data)
		for d.Next() {

			// Decode measurement name
			measurement, err := d.Measurement()
			if err != nil {
				msg := "_NatsReceive: Failed to decode measurement: " + err.Error()
				cclog.ComponentError(r.name, msg)
				return
			}

			// Decode tags
			tags := make(map[string]string)
			for {
				key, value, err := d.NextTag()
				if err != nil {
					msg := "_NatsReceive: Failed to decode tag: " + err.Error()
					cclog.ComponentError(r.name, msg)
					return
				}
				if key == nil {
					break
				}
				tags[string(key)] = string(value)
			}

			// Decode fields
			fields := make(map[string]interface{})
			for {
				key, value, err := d.NextField()
				if err != nil {
					msg := "_NatsReceive: Failed to decode field: " + err.Error()
					cclog.ComponentError(r.name, msg)
					return
				}
				if key == nil {
					break
				}
				fields[string(key)] = value.Interface()
			}

			// Decode time stamp
			t, err := d.Time(influx.Nanosecond, time.Time{})
			if err != nil {
				msg := "_NatsReceive: Failed to decode time: " + err.Error()
				cclog.ComponentError(r.name, msg)
				return
			}

			y, err := lp.NewMessage(
				string(measurement),
				tags,
				nil,
				fields,
				t,
			)
			if err == nil {
				m, err := r.mp.ProcessMessage(y)
				if err == nil && m != nil && r.sink != nil {
					r.sink <- m
				}
			}
		}
	}
}

// Close closes the connection to the NATS server
func (r *NatsReceiver) Close() {
	if r.nc != nil {
		cclog.ComponentDebug(r.name, "CLOSE")
		r.nc.Close()
	}
}

// NewNatsReceiver creates a new Receiver which subscribes to messages from a NATS server
func NewNatsReceiver(name string, config json.RawMessage) (Receiver, error) {
	var uinfo nats.Option = nil
	r := new(NatsReceiver)
	r.name = fmt.Sprintf("NatsReceiver(%s)", name)

	// Read configuration file, allow overwriting default config
	r.config.Addr = "localhost"
	r.config.Port = "4222"
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(r.config.Addr) == 0 ||
		len(r.config.Port) == 0 ||
		len(r.config.Subject) == 0 {
		return nil, errors.New("not all configuration variables set required by NatsReceiver")
	}
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	r.mp = p
	if len(r.config.MessageProcessor) > 0 {
		err = r.mp.FromConfigJSON(r.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}

	// Set metadata
	// r.meta = map[string]string{
	// 	"source": r.name,
	// }
	r.mp.AddAddMetaByCondition("true", "source", r.name)

	if len(r.config.User) > 0 && len(r.config.Password) > 0 {
		uinfo = nats.UserInfo(r.config.User, r.config.Password)
	} else if len(r.config.NkeyFile) > 0 {
		_, err := os.Stat(r.config.NkeyFile)
		if err == nil {
			uinfo = nats.UserCredentials(r.config.NkeyFile)
		} else {
			cclog.ComponentError(r.name, "NKEY file", r.config.NkeyFile, "does not exist: %v", err.Error())
			return nil, err
		}
	}

	// Connect to NATS server
	url := fmt.Sprintf("nats://%s:%s", r.config.Addr, r.config.Port)
	cclog.ComponentDebug(r.name, "NewNatsReceiver", url, "Subject", r.config.Subject)
	if nc, err := nats.Connect(url, uinfo); err == nil {
		r.nc = nc
	} else {
		r.nc = nil
		return nil, err
	}

	return r, nil
}
