package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hpcloud/tail"
	"github.com/nats-io/go-nats-streaming"
)

var fname string
var offset int64

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

func processFile(ch chan<- Msg) {
	// get last location
	dat, err := ioutil.ReadFile("seek")
	if err != nil {
		fmt.Printf("WARN: %s", err)
	} else {
		if len(dat) > 0 {
			offset, err = strconv.ParseInt(string(dat[:len(dat)-1]), 10, 64)
			if err != nil {
				fmt.Println("Error parsing offset:", err)
			}
		}
	}

	t, err := tail.TailFile(fname, tail.Config{Follow: true, ReOpen: true, MustExist: false, Location: &tail.SeekInfo{Offset: offset, Whence: 0}})
	if err != nil {
		fmt.Printf("Err: %s", err)
	}

	for line := range t.Lines {

		msg, err := lineMsg(line.Text)
		if err != nil {
			fmt.Println("Err lineToObject: ", err)
		}
		offset, err = t.Tell()
		if err != nil {
			fmt.Println("Tell offset error: ", err)
		}
		ch <- msg
	}
}

func stanproducer(p string) (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", p, name), nil
}

func main() {
	var stanserver string
	var stanclustername string
	var producer string
	var username string
	var password string

	flag.StringVar(&fname, "file", "", "File to tail")
	flag.StringVar(&stanclustername, "cluster", "", "Stan Cluster Name")
	flag.StringVar(&stanserver, "server", "", "Stan Server Name")
	flag.StringVar(&producer, "producer", "", "Producer Name")
	flag.StringVar(&username, "username", "icinga", "username")
	flag.StringVar(&password, "password", "password", "password")
	flag.Parse()

	if fname == "" {
		fmt.Println("Need filename to tail")
		os.Exit(1)
	}

	if stanserver == "" {
		fmt.Println("Need stan server to connect")
		os.Exit(1)
	}

	if stanclustername == "" {
		fmt.Println("Need stan clustername to connect")
		os.Exit(1)
	}

	producername, err := stanproducer(producer)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	msgch := make(chan Msg)
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// read file
	go processFile(msgch)

	//wait for signal and write offset to seek file
	go func() {
		sig := <-sigs
		fmt.Printf("Got %s, will stop now", sig)

		f, err := os.Create("seek")
		if err != nil {
			fmt.Println("Open offset file error: ", err)
		}
		defer f.Close()
		w, err := f.WriteString(fmt.Sprintf("%d\n", offset))
		if err != nil {
			fmt.Println("Error writing file", err)
		}
		fmt.Printf("wrote %d bytes\n", w)
		f.Sync()
		done <- true
	}()

	// Process Messages
	go func() {

		sc, err := stan.Connect(stanclustername, producername, stan.NatsURL(fmt.Sprintf("nats://%s:%s@%s:4222", username, password, stanserver)))
		if err != nil {
			fmt.Printf("Error connecting to nats: %s", err)
			os.Exit(0)
		}

		for msg := range msgch {
			fmt.Printf("time: %s, type: %s \n", msg.Stamp, msg.MsgType)
			b, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("Error marshaling MSG %s: %s", msg.Msg, err)
			}
			sc.Publish("icinga", b)
		}
	}()

	<-done
	fmt.Println("Exiting")
}
