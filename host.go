package main

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"sync"
)

type Host struct {
	config      HostCfg
	handlersCfg *HandlersCfgs
	checksCfg   *ChecksCfgs
	log         *log.Logger
	checks      []*Check
	client      *ssh.Client
	connMutex   *sync.Mutex
	sshCfg      *ssh.ClientConfig
	connecting  bool
	connected   bool
	reconnects  int64
	keepaliveCc ChecksCfg
}

type HostStatus struct {
	Host       string         `json:"host"`
	Reconnects int64          `json:"reconnects"`
	Connected  bool           `json:"connected"`
	Connecting bool           `json:"connecting"`
	Checks     []*CheckStatus `json:"checks"`
}

func NewHost(hst HostCfg, hnd *HandlersCfgs, c *ChecksCfgs, l *log.Logger) *Host {

	var connMutex = &sync.Mutex{}

	keepaliveCc := ChecksCfg{
		Name:        "keepalive",
		Script:      "true",
		Interval:    15,
		Description: "Keepalive check to ensure host is still reachable.",
	}

	h := &Host{
		config:      hst,
		handlersCfg: hnd,
		checksCfg:   c,
		log:         l,
		connMutex:   connMutex,
		connecting:  false,
		connected:   false,
		keepaliveCc: keepaliveCc,
	}

	return h
}

func (h *Host) start() error {
	err := h.getSSHConfig()
	if err != nil {
		return err
	}
	err = h.connect()
	if err != nil {
		return err
	}

	h.findChecks()

	err = h.pushChecks()
	if err != nil {
		return err
	}

	// Add keep alive check
	keepaliveCheck := NewCheck(h.keepaliveCc, h.handlersCfg, h, "true", h.log)
	h.checks = append(h.checks, keepaliveCheck)
	h.log.Printf("%s :: My checks %v", h.config.Host, h.checks)

	err = h.startChecks()

	return nil
}

func (h *Host) stop() error {
	h.log.Printf("%s stopping...", h.config.Host)
	h.stopChecks()
	h.client.Close()
	return nil
}

func (h *Host) findChecks() {
	for _, checkCfg := range h.checksCfg.Checks {
		//h.log.Printf("checkCfg %v", checkCfg)
		for _, sub := range h.config.Subscriptions {
			if strInSlice(sub, checkCfg.Subscribers) {
				s := h.config.Directory + "/" + filepath.Base(checkCfg.Script)
				c := NewCheck(checkCfg, h.handlersCfg, h, s, h.log)
				h.checks = append(h.checks, c)
				break // Break once this check is added
			}
		}
	}
}

func (h *Host) pushChecks() error {

	for _, check := range h.checks {
		pusher, err := NewFilePusher(h.config.User, h.config.Key, h.config.Host, h.config.Port, h.log)
		if err != nil {
			h.log.Printf("Unable to connect: %v", err)
			return err
		}
		defer pusher.Close()
		h.log.Printf("%s :: Pushing %v", h.config.Host, check.script)
		err = pusher.PushFile(check.config.Script, check.script, "0755")
		if err != nil {
			h.log.Printf("Unable to push file: %v", err)
			return err
		}
	}
	return nil
}

func (h *Host) getSSHConfig() error {
	key, err := ioutil.ReadFile(h.config.Key)
	if err != nil {
		h.log.Printf("unable to read private key: %v", err)
		return err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		h.log.Printf("unable to parse private key: %v", err)
		return err
	}

	h.sshCfg = &ssh.ClientConfig{
		User: h.config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return err
}

func (h *Host) connect() error {
	var err error
	// Bail if another thread has already calling this
	if h.connecting {
		return errors.New("Already connecting")
	}
	// Protection for multiple reconnect attempts
	h.connMutex.Lock()
	h.connecting = true
	h.connected = false
	if h.client != nil {
		h.client.Close()
		h.reconnects++
	}
	h.client, err = ssh.Dial("tcp", h.config.Host+":"+strconv.Itoa(h.config.Port), h.sshCfg)
	// Protection for multiple reconnect attempts
	h.connecting = false
	h.connMutex.Unlock()
	if err != nil {
		h.log.Printf("unable to connect: %v", err)
		return err
	}
	h.connected = true
	return err
}

func (h *Host) startChecks() error {
	for _, c := range h.checks {
		h.log.Printf("%s :: Starting check %v", h.config.Host, c.config.Script)
		c.start()
	}
	return nil
}

func (h *Host) stopChecks() error {
	for _, c := range h.checks {
		c.stop()
	}
	return nil
}

func (h *Host) getStatus() *HostStatus {
	var checks []*CheckStatus
	for _, c := range h.checks {
		cs := c.getCheckStatus()
		checks = append(checks, cs)
	}

	return &HostStatus{
		Host:       h.config.Host,
		Reconnects: h.reconnects,
		Connected:  h.connected,
		Connecting: h.connecting,
		Checks:     checks,
	}
}
