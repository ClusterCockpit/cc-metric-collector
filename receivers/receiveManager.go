package receivers

import (
	"encoding/json"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"log"
	"os"
	"sync"
)

var AvailableReceivers = map[string]Receiver{
	"nats": &NatsReceiver{},
}

type receiveManager struct {
	inputs []Receiver
	output chan lp.CCMetric
	done   chan bool
	wg     *sync.WaitGroup
	config []ReceiverConfig
}

type ReceiveManager interface {
	Init(wg *sync.WaitGroup, receiverConfigFile string) error
	AddInput(rawConfig json.RawMessage) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

func (rm *receiveManager) Init(wg *sync.WaitGroup, receiverConfigFile string) error {
	rm.inputs = make([]Receiver, 0)
	rm.output = nil
	rm.done = make(chan bool)
	rm.wg = wg
	rm.config = make([]ReceiverConfig, 0)
	configFile, err := os.Open(receiverConfigFile)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	var rawConfigs []json.RawMessage
	err = jsonParser.Decode(&rawConfigs)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	for _, raw := range rawConfigs {
		log.Print("[ReceiveManager] ", string(raw))
		rm.AddInput(raw)
		//        if _, found := AvailableReceivers[k.Type]; !found {
		//            log.Print("[ReceiveManager] SKIP Config specifies unknown receiver 'type': ", k.Type)
		//            continue
		//        }
		//        r := AvailableReceivers[k.Type]
		//        err = r.Init(k)
		//        if err != nil {
		//            log.Print("[ReceiveManager] SKIP Receiver ", k.Type, " cannot be initialized: ", err.Error())
		//            continue
		//        }
		//        rm.inputs = append(rm.inputs, r)
	}
	return nil
}

func (rm *receiveManager) Start() {
	rm.wg.Add(1)

	for _, r := range rm.inputs {
		log.Print("[ReceiveManager] START ", r.Name())
		r.Start()
	}
	log.Print("[ReceiveManager] STARTED\n")
	//    go func() {
	//        for {
	//ReceiveManagerLoop:
	//            select {
	//            case <- rm.done:
	//                log.Print("ReceiveManager done\n")
	//                rm.wg.Done()
	//                break ReceiveManagerLoop
	//            default:
	//                for _, c := range rm.inputs {
	//ReceiveManagerInputLoop:
	//                    select {
	//                    case <- rm.done:
	//                        log.Print("ReceiveManager done\n")
	//                        rm.wg.Done()
	//                        break ReceiveManagerInputLoop
	//                    case p := <- c:
	//                        log.Print("ReceiveManager: ", p)
	//                        rm.output <- p
	//                    default:
	//                    }
	//                }
	//            }
	//        }
	//    }()
	//    for _, r := range rm.inputs {
	//        r.Close()
	//    }
}

func (rm *receiveManager) AddInput(rawConfig json.RawMessage) error {
	var config ReceiverConfig
	err := json.Unmarshal(rawConfig, &config)
	if err != nil {
		log.Print("[ReceiveManager] SKIP ", config.Type, " JSON config error: ", err.Error())
		log.Print(err.Error())
		return err
	}
	if _, found := AvailableReceivers[config.Type]; !found {
		log.Print("[ReceiveManager] SKIP ", config.Type, " unknown receiver: ", err.Error())
		return err
	}
	r := AvailableReceivers[config.Type]
	err = r.Init(config)
	if err != nil {
		log.Print("[ReceiveManager] SKIP ", r.Name(), " initialization failed: ", err.Error())
		return err
	}
	rm.inputs = append(rm.inputs, r)
	rm.config = append(rm.config, config)
	return nil
}

func (rm *receiveManager) AddOutput(output chan lp.CCMetric) {
	rm.output = output
	for _, r := range rm.inputs {
		r.SetSink(rm.output)
	}
}

func (rm *receiveManager) Close() {
	for _, r := range rm.inputs {
		log.Print("[ReceiveManager] CLOSE ", r.Name())
		r.Close()
	}
	rm.wg.Done()
	log.Print("[ReceiveManager] CLOSE\n")
	log.Print("[ReceiveManager] EXIT\n")
}

func New(wg *sync.WaitGroup, receiverConfigFile string) (ReceiveManager, error) {
	r := &receiveManager{}
	err := r.Init(wg, receiverConfigFile)
	if err != nil {
		return nil, err
	}
	return r, err
}
