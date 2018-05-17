package main

import (
	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
	"log"
	"os"
	"strconv"
)

type FilePusher struct {
	User   string
	Key    string
	Host   string
	Port   int
	log    *log.Logger
	client scp.Client
}

func NewFilePusher(user string, key string, host string, port int, logger *log.Logger) (*FilePusher, error) {

	/*
		hk, err := getSSHHostKey(host)
		if err != nil {
			logger.Printf("Failed to get host key %v", err)
			return nil, err
		}
	*/

	clientConfig, err := auth.PrivateKey(user, key, ssh.InsecureIgnoreHostKey())
	if err != nil {
		logger.Printf("Failed create private key %v", err)
		return nil, err
	}

	client := scp.NewClient(host+":"+strconv.Itoa(port), &clientConfig)
	err = client.Connect()
	if err != nil {
		logger.Printf("Couldn't establish a connection to %s:%d :: %v", host, port, err)
		return nil, err
	}

	f := &FilePusher{
		User:   user,
		Key:    key,
		Host:   host,
		log:    logger,
		client: client,
	}

	return f, nil
}

func (f *FilePusher) PushFile(local string, remote string, perm string) error {
	// Open a file
	l, err := os.Open(local)
	if err != nil {
		f.log.Printf("Failed to open local file %s: %v", local, err)
		return err
	}
	defer l.Close()
	f.client.CopyFile(l, remote, perm)
	return nil
}

func (f *FilePusher) Close() {
	f.client.Session.Close()
}
