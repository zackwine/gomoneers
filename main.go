package main

import (
	"flag"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"
)

var (
	hostsYml    = flag.String("hosts", "conf/hosts.yml", "The path to hosts file (yaml) defining hosts to monitor.")
	handlersYml = flag.String("handlers", "conf/handlers.yml", "The path to handlers file (yaml) defining available handlers.")
	checksYml   = flag.String("checks", "conf/checks.yml", "The path to checks file (yaml) defining checks to run on hosts.")
	tosyslog    = flag.Bool("syslog", false, "Log to syslog")
)

func main() {

	// Parse cmd args
	flag.Parse()

	// Setup logging
	var logwriter io.Writer
	if *tosyslog {
		logwriter, _ = syslog.New(syslog.LOG_NOTICE, "gomoneers")
	}
	if logwriter == nil {
		logwriter = os.Stdout
	}
	logger := log.New(logwriter, "", log.LstdFlags)

	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	exit := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Println("\nReceived Signal: %v", sig)
		exit <- true
	}()

	logger.Printf("Starting with hosts (%s) handlers (%s) checks (%s)...", *hostsYml, *handlersYml, *checksYml)

	gomoneers, err := NewGoMonErrs(*hostsYml, *handlersYml, *checksYml, logger)
	if err != nil {
		log.Fatalf("Failed to create gomoneers: %v", err)
		os.Exit(1)
	}

	gomoneers.start()

	// Wait for signal (cntl-c/kill)
	<-exit

	logger.Printf("Shutting down...")
	gomoneers.stop()

}
