package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

var fname string

// Msg is a raw message
type Msg struct {
	Stamp   time.Time `json:"timestamp"`
	MsgType string    `json:"type"`
	Msg     json.RawMessage
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

func msgTime(s string) (time.Time, error) {
	i, err := strconv.ParseInt(strings.Trim(s, "[]"), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("Err: %s", err)
	}
	return time.Unix(i, 0), nil
}

func splitMsg(s string) (rawtime, msgtype, msg string) {
	headType := strings.SplitN(s, ":", 2)
	rawtimeType := strings.SplitN(headType[0], " ", 2)

	return rawtimeType[0], rawtimeType[1], headType[1]

}

func hostMessage(t, s string) (HostMsg, error) {
	sp := strings.SplitN(s, ";", 5)

	i, err := strconv.ParseInt(sp[3], 10, 64)
	if err != nil {
		return HostMsg{}, fmt.Errorf("Err: failed to parse HostMsg.StateCount %s", err)
	}
	return HostMsg{
		SubType:    t,
		HostName:   sp[0],
		State:      sp[1],
		StateType:  sp[2],
		StateCount: i,
		Message:    sp[4],
	}, nil
}

func serviceMessage(t, s string) (ServiceMsg, error) {
	sp := strings.SplitN(s, ";", 6)

	i, err := strconv.ParseInt(sp[4], 10, 64)
	if err != nil {
		return ServiceMsg{}, fmt.Errorf("Err: failed to parse ServiceMsg.StateCount %s", err)
	}
	return ServiceMsg{
		SubType:     t,
		HostName:    sp[0],
		ServiceName: sp[1],
		State:       sp[2],
		StateType:   sp[3],
		StateCount:  i,
		Message:     sp[5],
	}, nil

}

/*
CURRENT HOST STATE:     www.meteopower.com;DOWN;SOFT;1;<Terminated by signal 15 (Terminated).>
HOST ALERT:             www.meteopower.com;UP;HARD;1;PING OK - Packet loss = 16%, RTA = 2.39 ms
HOST FLAPPING ALERT:    m6play02;STARTED; Checkable appears to have started flapping (100% change >= 30% threshold)
HOST DOWNTIME ALERT:    mcrender08;STARTED; Checkable has entered a period of scheduled downtime.

CURRENT SERVICE STATE:  www.meteopower.com;www domains;OK;HARD;1;HTTP OK: Status line output matched "HTTP/1.1 200 OK" - 315 bytes in 0.061 second response time
SERVICE ALERT:          mg-carfeed-prelive01.thdmz.pamgservices.net;SNMP_Storage_fixed;UNKNOWN;SOFT;1;ERROR: General time-out (Alarm signal)
SERVICE DOWNTIME ALERT: mguk-mysql-2.lb.meteogroup.net;SNMP_Storage_fixed;STOPPED; Checkable has exited from a period of scheduled downtime.
SERVICE FLAPPING ALERT: orfdata03;MySQL;STARTED; Checkable appears to have started flapping (100% change >= 30% threshold)

EXTERNAL COMMAND:       PROCESS_SERVICE_CHECK_RESULT;jmsmaster.ukjms.pamgservices.net;Hydrocast SQL (JMS-UK 637);0;Job ran successfully

*/

func lineMsg(line string) (Msg, error) {
	var obj = Msg{}
	t, msgtype, msg := splitMsg(line)

	time, err := msgTime(t)
	if err != nil {
		return Msg{}, err
	}
	obj.Stamp = time

	switch msgtype {
	case "CURRENT HOST STATE":
		obj.MsgType = "HostMsg"

		hm, err := hostMessage(msgtype, msg)
		if err != nil {
			return Msg{}, err
		}
		obj.Msg, err = json.Marshal(hm)
		if err != nil {
			return obj, fmt.Errorf("Error marshal %s , %s", msg, err)
		}
	case "HOST ALERT":
		obj.MsgType = "HostMsg"

		hm, err := hostMessage(msgtype, msg)
		if err != nil {
			return Msg{}, err
		}
		obj.Msg, err = json.Marshal(hm)
		if err != nil {
			return obj, fmt.Errorf("Error marshal %s , %s", msg, err)
		}
	case "CURRENT SERVICE STATE":
		obj.MsgType = "ServiceMsg"

		hm, err := serviceMessage(msgtype, msg)
		if err != nil {
			return Msg{}, err
		}
		obj.Msg, err = json.Marshal(hm)
		if err != nil {
			return obj, fmt.Errorf("Error marshal %s , %s", msg, err)
		}
	case "SERVICE ALERT":
		obj.MsgType = "ServiceMsg"

		hm, err := serviceMessage(msgtype, msg)
		if err != nil {
			return Msg{}, err
		}
		obj.Msg, err = json.Marshal(hm)
		if err != nil {
			return obj, fmt.Errorf("Error marshal %s , %s", msg, err)
		}
	default:
		fmt.Println("TBD")
	}

	return obj, nil
}

func main() {
	var offset int64
	flag.StringVar(&fname, "file", "", "File to tail")
	flag.Parse()

	// get last location
	dat, err := ioutil.ReadFile("seek")
	if err != nil {
		fmt.Printf("WARN: %s", err)
	}
	if len(dat) > 0 {
		offset, err = strconv.ParseInt(string(dat[:len(dat)-1]), 10, 64)
		if err != nil {
			fmt.Println("Error parsing offset:", err)
		}
	}

	t, err := tail.TailFile(fname, tail.Config{Follow: true, ReOpen: true, MustExist: false, Location: &tail.SeekInfo{Offset: offset, Whence: 0}})
	if err != nil {
		fmt.Printf("Err: %s", err)
	}
	//TODO write seek under all circumstances
	// but this does not work
	////f, err := os.Create("seek")
	////defer func() {
	////	offset, err = t.Tell()
	////	if err != nil {
	////		fmt.Println("Tell offset error: ", err)
	////	}
	////	f.WriteString(string(offset))
	////	f.Sync()
	////	f.Close()
	////	//err = ioutil.WriteFile("seek", []byte(string(offset)), 0644)
	////	//if err != nil {
	////	//	fmt.Println("Write offset error: ", err)
	////	//}
	////}()

	for line := range t.Lines {
		// it does not work ...

		msg, err := lineMsg(line.Text)
		if err != nil {
			fmt.Println("Err lineToObject: ", err)
		}

		//fmt.Printf("time: %s, type: %s , message: %s\n", msg.Stamp, msg.MsgType, string(msg.Msg))
		fmt.Printf("time: %s, type: %s \n", msg.Stamp, msg.MsgType)
	}
	fmt.Println("Exiting")
}
