package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the private JSON configuration")
	listen := flag.String("listen", "127.0.0.1:8090", "HTTP listen address")
	initialize := flag.Bool("init", false, "write a new configuration template")
	check := flag.Bool("check", false, "validate the configuration and exit")
	flag.Parse()

	if err := run(*configPath, *listen, *initialize, *check); err != nil {
		log.Fatal(err)
	}
}

func run(configPath, listen string, initialize, check bool) error {
	if initialize && check {
		return errors.New("select either -init or -check")
	}
	if initialize {
		if err := initializeConfig(configPath); err != nil {
			return err
		}
		fmt.Printf("Created %s. Set the server, port and password for each node.\n", configPath)
		return nil
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	if check {
		fmt.Printf("Configuration valid (%d nodes).\n", len(cfg.Nodes))
		return nil
	}

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           subscriptionHandler(cfg),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}
	log.Printf("Subscription server listening on http://%s", listener.Addr())
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
