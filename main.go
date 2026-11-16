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
	var opts options
	flag.StringVar(&opts.configPath, "config", "config.json", "path to the private JSON configuration")
	flag.StringVar(&opts.listen, "listen", "127.0.0.1:8090", "subscription HTTP listen address")
	flag.BoolVar(&opts.initialize, "init", false, "write a new configuration template")
	flag.BoolVar(&opts.check, "check", false, "validate the configuration and exit")
	flag.StringVar(&opts.exportNode, "export-node", "", "export a sing-box server configuration for this node ID")
	flag.StringVar(&opts.out, "out", "", "new private file to write with -export-node")
	flag.StringVar(&opts.nodeBind, "node-bind", "127.0.0.1", "local IP address to bind in the exported server configuration")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

type options struct {
	configPath string
	listen     string
	initialize bool
	check      bool
	exportNode string
	out        string
	nodeBind   string
}

func run(opts options) error {
	if (opts.initialize && opts.check) || (opts.exportNode != "" && (opts.initialize || opts.check)) {
		return errors.New("select only one of -init, -check or -export-node")
	}
	if (opts.out == "") != (opts.exportNode == "") {
		return errors.New("-export-node and -out must be used together")
	}
	if opts.initialize {
		if err := initializeConfig(opts.configPath); err != nil {
			return err
		}
		fmt.Printf("Created %s. Set the server, port and password for each node.\n", opts.configPath)
		return nil
	}

	cfg, err := loadConfig(opts.configPath)
	if err != nil {
		return err
	}
	if opts.check {
		fmt.Printf("Configuration valid (%d nodes).\n", len(cfg.Nodes))
		return nil
	}
	if opts.exportNode != "" {
		if err := exportNodeConfig(cfg, opts.exportNode, opts.nodeBind, opts.out); err != nil {
			return err
		}
		fmt.Printf("Created sing-box node configuration %s.\n", opts.out)
		return nil
	}

	listener, err := net.Listen("tcp", opts.listen)
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
