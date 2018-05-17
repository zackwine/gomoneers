package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type HostCfg struct {
	Name          string
	Host          string
	Port          int
	User          string
	Key           string
	Subscriptions []string
	Directory     string
}

type HostCfgs struct {
	Hosts []HostCfg `hosts`
}

func NewHostCfgs(filename string) (*HostCfgs, error) {
	t := &HostCfgs{}
	s, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed reading file (%s): %v", filename, err)
		return nil, err
	}
	err = yaml.Unmarshal(s, t)
	if err != nil {
		fmt.Printf("Failed in Unmarshal for file (%s): %v", filename, err)
		return nil, err
	}
	return t, err
}
