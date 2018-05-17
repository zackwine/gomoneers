package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type ChecksCfg struct {
	Name        string
	Script      string
	Args        string
	Interval    int
	Description string
	Subscribers []string
	Handlers    []string
}

type ChecksCfgs struct {
	Checks []ChecksCfg `checks`
}

func NewChecksCfgs(filename string) (*ChecksCfgs, error) {
	t := &ChecksCfgs{}
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
