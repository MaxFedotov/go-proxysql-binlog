package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	uuid "github.com/satori/go.uuid"
	log "github.com/siddontang/go-log/log"
	"github.com/siddontang/go-mysql/replication"
)

func init() {
	logHandler, _ := log.NewStreamHandler(os.Stdout)
	logger := log.New(logHandler, 1|4)
	log.SetDefaultLogger(logger)
}

var (
	Config             = newConfiguration()
	Version, GitCommit string
	wg                 sync.WaitGroup
)

func main() {
	flagSet := flag.NewFlagSet("proxysq_binlog", flag.ExitOnError)
	configFile := flagSet.String("config", "/etc/proxysql-binlog.cnf", "Path to config file")
	debug := flagSet.Bool("debug", false, "Debug mode")
	version := flagSet.Bool("version", false, "Print version")

	flagSet.Parse(os.Args[1:])

	if *version {
		fmt.Println("Version:\t", Version)
		fmt.Println("Git commit:\t", GitCommit)
		os.Exit(0)
	}

	log.SetLevel(log.LevelInfo)

	if _, err := toml.DecodeFile(*configFile, &Config); err != nil {
		log.Fatalf("unable to decode coniguration file %s: %v", *configFile, err)
	}

	if Config.General.LogFile != "" {
		logFile, err := os.OpenFile(Config.General.LogFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0640)
		if err != nil {
			log.Fatalf("unable to open log file %s: %v", Config.General.LogFile, err)
		}
		mw := io.MultiWriter(os.Stdout, logFile)
		logHandler, _ := log.NewStreamHandler(mw)
		logger := log.New(logHandler, 1|4)
		log.SetDefaultLogger(logger)
		defer logHandler.Close()
	}

	if strings.ToLower(Config.General.LogLevel) == "debug" {
		*debug = true
	}

	if *debug {
		log.SetLevel(log.LevelDebug)
	}

	dispatcher := NewDispatcher()
	quit := make(chan os.Signal)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	err := ListenAndServeTCP(Config.General.ListenAddress, dispatcher, ctx)
	if err != nil {
		log.Fatalf("unable to start TCP server: %v", err)
	}

	metrics := NewMetricsServer()

	go func() {
	createReader:
		binlogReader, err := NewBinlogReader()
		if err != nil {
			log.Fatalf("unable to start binlog reader: %v", err)
		}
		binlogStreamer, err := binlogReader.StartSync()
		if err != nil {
			log.Fatalf("unable to start binlog streamer: %v", err)
		}
		wg.Add(1)
		for {
			ev, err := binlogStreamer.GetEvent(ctx)
			if err != nil {
				if err == ctx.Err() {
					log.Info("stopping BinlogReader")
					binlogReader.Close()
					wg.Done()
					return
				}
				log.Errorf("error during getting binlog event: %v", err)
				readerErrors.Inc()
				binlogReader.Close()
				wg.Done()
				goto createReader
			}
			if ev.Header.EventType.String() == "GTIDEvent" {
				event := &GTIDEvent{}
				gtidEvent, _ := ev.Event.(*replication.GTIDEvent)
				u, err := uuid.FromBytes(gtidEvent.SID)
				if err != nil {
					log.Errorf("Unable to parse GTID: %v", err)
					readerErrors.Inc()
					continue
				}
				event.gtidSID = UUID(u)
				event.gtidGNO = gtidEvent.GNO
				log.Debugf("got gtid event: %s", fmt.Sprintf("%s:%d", event.gtidSID.String(), event.gtidGNO))
				gtidProcessed.Inc()
				dispatcher.Events <- event
			}
		}
	}()
	go signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	log.Infof("got %s signal. Stopping", <-quit)
	cancel()
	metrics.Close()
	wg.Wait()
	os.Exit(0)
}
