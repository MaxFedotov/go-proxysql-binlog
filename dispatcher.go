package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"

	uuid "github.com/satori/go.uuid"
	log "github.com/siddontang/go-log/log"
)

type Dispatcher struct {
	Events         chan *GTIDEvent
	NewClients     chan chan []byte
	ClosingClients chan chan []byte
	Clients        map[chan []byte]UUID
}

type GTIDEvent struct {
	gtidSID UUID
	gtidGNO int64
}

type UUID uuid.UUID

func (u UUID) String() string {
	buf := make([]byte, 32)
	hex.Encode(buf[0:32], u[0:])
	return string(buf)
}

func NewDispatcher() (dispatcher *Dispatcher) {
	dispatcher = &Dispatcher{
		Events:         make(chan *GTIDEvent, 1),
		NewClients:     make(chan chan []byte),
		ClosingClients: make(chan chan []byte),
		Clients:        make(map[chan []byte]UUID),
	}
	go dispatcher.process()
	return
}

func (dispatcher *Dispatcher) process() {
	for {
		var gtidEvent *GTIDEvent
		select {
		case s := <-dispatcher.NewClients:
			dispatcher.Clients[s] = UUID(uuid.Nil)
			log.Infof("client added to dispatcher. Total clients: %d", len(dispatcher.Clients))
			gtidSet, _ := Config.getMasterGTIDSet()
			var gtidExecuted []string
			for _, gtid := range strings.Split(gtidSet.String(), ",") {
				matched, _ := regexp.MatchString(`^.*:\d$`, gtid)
				if matched {
					gtid += "-1"
				}
				gtidExecuted = append(gtidExecuted, gtid)
			}
			s <- []byte(fmt.Sprintf("ST=%s\n", strings.Join(gtidExecuted, ",")))
		case s := <-dispatcher.ClosingClients:
			delete(dispatcher.Clients, s)
			log.Infof("client removed from dispatcher. Total clients: %d", len(dispatcher.Clients))
		case gtidEvent = <-dispatcher.Events:
			for client, clientGtidSID := range dispatcher.Clients {
				if clientGtidSID != gtidEvent.gtidSID {
					client <- []byte(fmt.Sprintf("I1=%s:%d\n", gtidEvent.gtidSID.String(), gtidEvent.gtidGNO))
					dispatcher.Clients[client] = gtidEvent.gtidSID
				} else {
					client <- []byte(fmt.Sprintf("I2=%d\n", gtidEvent.gtidGNO))
				}

			}
		}
	}
}

func (dispatcher *Dispatcher) ServeTCP(conn net.Conn) error {
	eventsChan := make(chan []byte)
	dispatcher.NewClients <- eventsChan
	defer func() {
		log.Infof("closing connection from %v", conn.RemoteAddr())
		conn.Close()
		clientsConnected.Dec()
		dispatcher.ClosingClients <- eventsChan
	}()

	w := bufio.NewWriter(conn)
	for {
		_, err := fmt.Fprintf(w, "%s", <-eventsChan)
		if err != nil {
			log.Errorf("unable to process event: %v", err)
			clientErrors.Inc()
			return nil
		}
		err = w.Flush()
		if err != nil {
			log.Errorf("unable to send event to client %s: %v", conn.RemoteAddr(), err)
			clientErrors.Inc()
			return nil
		}
	}
}
