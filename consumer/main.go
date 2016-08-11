package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	elastic "gopkg.in/olivere/elastic.v2"

	"github.com/nats-io/go-nats-streaming"
)

// Msg is a raw message
type Msg struct {
	Stamp   time.Time `json:"timestamp"`
	MsgType string    `json:"type"`
	Msg     []byte    `json:"msg"`
}

func (m *Msg) String() string {
	nano := time.Time(m.Stamp).UnixNano()
	t := nano / 1000000
	return fmt.Sprintf("{\"timestamp\": \"%d\", \"type\": \"%s\" , \"msg\": %s }", t, m.MsgType, string(m.Msg))
}

func toElasticsearch(client *elastic.Client, j *Msg) error {
	indexName := j.Stamp.Format("logstash-2006.01.02")

	indexMapping := `{ "mappings": { "IcingaLog": { "properties": { "timestamp" : { "type": "date" }}}}}`

	exists, err := client.IndexExists(indexName).Do()
	if err != nil {
		return err
	}

	if !exists {
		res, err := client.CreateIndex(indexName).
			Body(indexMapping).
			Do()

		if err != nil {
			return err
		}
		if !res.Acknowledged {
			return errors.New("CreateIndex was not acknowledged. Check that timeout value is correct.")
		}
	}
	return addLogsToIndex(client, indexName, fmt.Sprintf("%s\n", j))
}

func addLogsToIndex(client *elastic.Client, index, j string) error {
	_, err := client.Index().
		Index(index).
		Type("IcingaLog").
		BodyString(j).Do()
	if err != nil {
		return err
	}
	return nil
}

func sendMsg(m *stan.Msg, i int, es *elastic.Client) {
	var imsg Msg
	//log.Printf("[#%d] Received on [%s]: '%s'\n", i, m.Subject, m)

	if err := json.Unmarshal(m.Data, &imsg); err != nil {
		log.Fatal(err)
	}

	if imsg.MsgType != "" {
		if err := toElasticsearch(es, &imsg); err != nil {
			fmt.Println(err)
		}
	}

}

func stanconsumer(p string) (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", p, name), nil
}

func main() {

	var clusterID string
	var stanserver string
	var username string
	var password string
	var consumer string
	var esURL string

	flag.StringVar(&clusterID, "cluster", "test", "Stan Cluster Name")
	flag.StringVar(&stanserver, "server", "", "Stan Server Name")
	flag.StringVar(&username, "username", "icinga", "username")
	flag.StringVar(&password, "password", "password", "password")
	flag.StringVar(&consumer, "consumer", "consumer", "Consumer Name")
	flag.StringVar(&esURL, "es", "http://localhost:9200", "Elasticsearch URL")
	flag.Parse()

	if stanserver == "" {
		fmt.Println("Need stan server to connect")
		os.Exit(1)
	}

	if clusterID == "" {
		fmt.Println("Need stan clustername to connect")
		os.Exit(1)
	}

	clientID, err := stanconsumer(consumer)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	URL := fmt.Sprintf("nats://%s:%s@%s:4222", username, password, stanserver)

	subj := "icinga"
	qgroup := ""
	durable := clientID + qgroup
	startOpt := stan.DeliverAllAvailable()
	unsubscribe := true

	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(URL))
	if err != nil {
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s", err, URL)
	}
	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", URL, clusterID, clientID)

	esclient, err := elastic.NewClient(elastic.SetURL(esURL))
	if err != nil {
		panic(err)
	}

	i := 0
	mcb := func(msg *stan.Msg) {
		i++
		sendMsg(msg, i, esclient)
	}

	sub, err := sc.QueueSubscribe(subj, qgroup, mcb, startOpt, stan.DurableName(durable))
	if err != nil {
		sc.Close()
		log.Fatal(err)
	}

	log.Printf("Listening on [%s], clientID=[%s], qgroup=[%s] durable=[%s]\n", subj, clientID, qgroup, durable)

	// Wait for a SIGINT (perhaps triggered by user with CTRL-C)
	// Run cleanup when signal is received
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for _ = range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			// Do not unsubscribe a durable on exit, except if asked to.
			if durable == "" || unsubscribe {
				sub.Unsubscribe()
			}
			sc.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}
