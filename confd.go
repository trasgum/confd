package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/kelseyhightower/confd/backends"
	"github.com/kelseyhightower/confd/log"
	"github.com/kelseyhightower/confd/resource/template"
)

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("confd %s (Git SHA: %s, Go Version: %s)\n", Version, GitSHA, runtime.Version())
		os.Exit(0)
	}
	if err := initConfig(); err != nil {
		log.Fatal(err.Error())
	}

	log.Info("Starting confd")

	storeClient, err := backends.New(backendsConfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	templateConfig.StoreClient = storeClient
	if onetime {
		if err := template.Process(templateConfig); err != nil {
			log.Fatal(err.Error())
		}
		os.Exit(0)
	}

	start:
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	var processor template.Processor
	switch {
	case config.Watch:
		processor = template.WatchProcessor(templateConfig, stopChan, doneChan, errChan)
	default:
		processor = template.IntervalProcessor(templateConfig, stopChan, doneChan, errChan, config.Interval)
	}
	go processor.Process()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	for {
		select {
		case err := <-errChan:
			log.Error(err.Error())
		case s := <-signalChan:
			switch s {
			case syscall.SIGUSR1:
				log.Info("Received %v. Reloading templates", s)
				close(stopChan)
				goto start
			case syscall.SIGINT, syscall.SIGTERM:
				log.Info(fmt.Sprintf("Captured %v. Exiting...", s))
				close(doneChan)
				os.Exit(0)
			default:
				log.Info("Captured %v. Continue...")
			}
		/*case <-doneChan:
			log.Info("Done...")
			os.Exit(0)*/
		}
	}
	log.Info("End...")
}
