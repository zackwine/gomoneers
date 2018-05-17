package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"
)

const (
	RESULTS_QUEUE_SIZE = 20
)

type Result struct {
	status     string
	exitstatus int
	stdout     []byte
	stderr     []byte
	executed   time.Time
}

type Check struct {
	config      ChecksCfg
	host        *Host
	client      *ssh.Client
	remote      string
	handlers    *HandlersCfgs
	script      string
	log         *log.Logger
	results     []*Result
	last        *Result
	stopchan    chan struct{}
	attempts    int64
	warnings    int64
	failures    int64
	unknowns    int64
	sshFailures int64
}

type CheckStatus struct {
	Name        string
	Ago         string
	Executed    int64
	Exitstatus  int
	Stdout      string
	Stderr      string
	Attempts    int64
	Warnings    int64
	Failures    int64
	Unknowns    int64
	SshFailures int64
}

func NewCheck(cc ChecksCfg, h *HandlersCfgs, host *Host, s string, l *log.Logger) *Check {
	c := &Check{
		config:   cc,
		host:     host,
		remote:   host.config.Host,
		handlers: h,
		script:   s,
		log:      l,
		results:  make([]*Result, RESULTS_QUEUE_SIZE),
	}

	return c
}

func (c *Check) start() error {
	var err error
	tick := time.NewTicker(time.Duration(c.config.Interval) * time.Second)
	c.stopchan = make(chan struct{})
	//Run initial check
	c.check()
	go func() {
		for {
			select {
			case <-tick.C:
				err = c.check()
				if err != nil {
					// attempt to reconnect if the check fails
					c.host.connect()
				}
			case <-c.stopchan:
				tick.Stop()
				return
			}
		}
	}()
	return nil
}

func (c *Check) stop() {
	close(c.stopchan)
}

func (c *Check) check() error {
	var stdout, stderr []byte
	var exitstatus int
	var executed time.Time

	c.log.Printf("[%v]:%s :: Checking", c.remote, c.script)
	c.attempts++

	if c.host.connected == false {
		c.sshFailures++
		return errors.New("Not connected")
	}

	session, err := c.host.client.NewSession()
	if err != nil {
		c.log.Printf("[%v]:%s :: Unable to open session: %v", c.remote, c.script, err)
		c.sshFailures++
		return err
	}

	fin := make(chan bool)
	sop, err := session.StdoutPipe()
	if err != nil {
		c.log.Printf("[%v]:%s :: Unable to setup stdout for session: %v", c.remote, c.script, err)
		c.sshFailures++
		return err
	}
	go func() {
		stdout, _ = ioutil.ReadAll(sop)
		fin <- true
	}()

	sep, err := session.StderrPipe()
	if err != nil {
		c.log.Printf("[%v]:%s :: Unable to setup stderr for session: %v", c.remote, c.script, err)
		c.sshFailures++
		return err
	}
	go func() {
		stderr, _ = ioutil.ReadAll(sep)
		fin <- true
	}()

	executed = time.Now()
	err = session.Run(c.script)

	// Wait on stdout and stderr reader routines to finish
	<-fin
	<-fin
	close(fin)

	//c.log.Printf("[%v]:%s :: Stdout %s", c.remote, c.script, stdout)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			c.log.Printf("[%v]:%s :: Check failed (%v) (%s) (%s) ", c.remote, c.script, err, stdout, stderr)
			exitstatus = exitErr.ExitStatus()
		} else {
			c.log.Printf("[%v]:%s :: SSH error : %v", c.remote, c.script, err)
			c.sshFailures++
			return err
		}
	}
	c.storeResult(exitstatus, stdout, stderr, executed)

	return nil
}

func (c *Check) storeResult(exitStatus int, stdout []byte, stderr []byte, executed time.Time) {
	var status string
	switch exitStatus {
	case 0:
		status = "OK"
	case 1:
		status = "WARNING"
		c.warnings++
	case 2:
		status = "FAILURE"
		c.failures++
	default:
		status = "UNKNOWN"
		c.unknowns++
	}
	e := &Result{
		status:     status,
		exitstatus: exitStatus,
		stdout:     stdout,
		stderr:     stderr,
		executed:   executed,
	}
	c.results = append(c.results, e)

	if c.last != nil && c.last.exitstatus != exitStatus {
		// Fire handlers

		// Build buffer of markdown
		var buffer bytes.Buffer
		buffer.WriteString(fmt.Sprintf("## %s %s\n\n", status, c.config.Name))
		buffer.WriteString(fmt.Sprintf("- *Host:* %v\n", c.remote))
		buffer.WriteString(fmt.Sprintf("- *Script:* %s\n", c.script))
		buffer.WriteString(fmt.Sprintf("- *Last Status:* %s\n\n", c.last.status))
		buffer.WriteString(fmt.Sprintf("- *Description:* %s\n\n", c.config.Description))
		buffer.WriteString(fmt.Sprintf("#### STDOUT\n```\n%s\n```\n\n", stdout))
		buffer.WriteString(fmt.Sprintf("#### STDERR\n```\n%s\n```\n\n", stderr))

		c.log.Printf("[%v]:%s :: Handle check status change: %d", c.remote, c.script, exitStatus)
		c.fireHandlers(buffer.String())
	}
	c.last = e
}

func (c *Check) fireHandlers(msg string) {
	c.log.Printf("[%v]:%s :: Firehandlers: %s", c.remote, c.script, msg)
	for _, checkHandler := range c.config.Handlers {
		for _, h := range c.handlers.Handlers {
			if h.Name == checkHandler {
				c.log.Printf("[%v]:%s :: Found matching handler: %s", c.remote, c.script, checkHandler)

				cmds := strings.Fields(h.Command)
				cmds = append(cmds, msg)
				out, err := exec.Command(cmds[0], cmds[1:]...).Output()
				if err != nil {
					c.log.Printf("Failed to execute handler (%s): %v", h.Command, err)
				}
				c.log.Printf("[%v]:%s :: %s :: (%s)", c.remote, c.script, checkHandler, out)
			}
		}
	}
}

func (c *Check) printResults() {
	for i, r := range c.results {
		if r != nil {
			c.log.Printf("%d - %v", i, r)
		}
	}
}

func (c *Check) getCheckStatus() *CheckStatus {
	s := &CheckStatus{
		Name: c.config.Name,
	}
	if c.last != nil {
		s.Exitstatus = c.last.exitstatus
		s.Stderr = string(c.last.stderr[:])
		s.Stdout = string(c.last.stdout[:])
		s.Executed = c.last.executed.Unix()
		s.Ago = humanize.Time((c.last.executed))
		s.Attempts = c.attempts
		s.Warnings = c.warnings
		s.Failures = c.failures
		s.Unknowns = c.unknowns
		s.SshFailures = c.sshFailures
	} else {
		s.Ago = "Never"
		s.Attempts = c.attempts
		s.SshFailures = c.sshFailures
	}
	return s
}
