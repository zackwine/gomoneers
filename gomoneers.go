package main

import (
	"log"
)

type GoMonErrs struct {
	hostsYml    string
	hostsCfg    *HostCfgs
	handlersYml string
	handlersCfg *HandlersCfgs
	checksYml   string
	checksCfg   *ChecksCfgs
	log         *log.Logger
	hosts       []*Host
	httpserver  *HttpServer
}

func NewGoMonErrs(hostsYml string, handlersYml string, checksYml string, logger *log.Logger) (*GoMonErrs, error) {

	hostsCfg, err := NewHostCfgs(hostsYml)
	if err != nil {
		logger.Printf("Failed to parse hosts config: %v", err)
		return nil, err
	}
	logger.Printf("Hosts Config: %v", hostsCfg)

	handlersCfg, err := NewHandlersCfgs(handlersYml)
	if err != nil {
		logger.Printf("Failed to parse handlers config: %v", err)
		return nil, err
	}
	logger.Printf("Handlers Config: %v", handlersCfg)

	checksCfg, err := NewChecksCfgs(checksYml)
	if err != nil {
		logger.Printf("Failed to parse checks config: %v", err)
		return nil, err
	}
	logger.Printf("Checks Config: %v", checksCfg)

	var hosts []*Host
	for _, hostCfg := range hostsCfg.Hosts {
		h := NewHost(hostCfg, handlersCfg, checksCfg, logger)
		hosts = append(hosts, h)
	}

	httpserver := NewHttpServer(":8080", logger)

	g := &GoMonErrs{
		hostsYml:    hostsYml,
		hostsCfg:    hostsCfg,
		handlersYml: handlersYml,
		handlersCfg: handlersCfg,
		checksYml:   checksYml,
		checksCfg:   checksCfg,
		log:         logger,
		hosts:       hosts,
		httpserver:  httpserver,
	}

	return g, nil
}

func (g *GoMonErrs) start() error {
	g.httpserver.startHttp(g)
	for _, host := range g.hosts {
		err := host.start()
		if err != nil {
			g.log.Printf("Failed to start host (%v): %v", host.config, err)
			return err
		}
	}
	return nil
}

func (g *GoMonErrs) stop() error {
	for _, host := range g.hosts {
		err := host.stop()
		if err != nil {
			g.log.Printf("Failed to stop host (%v): %v", host.config, err)
		}
	}
	g.httpserver.stopHttp()
	return nil
}

func (g *GoMonErrs) getStatus() []*HostStatus {
	var hs []*HostStatus
	for _, host := range g.hosts {
		s := host.getStatus()
		if s != nil {
			hs = append(hs, s)
		}
	}
	return hs
}
