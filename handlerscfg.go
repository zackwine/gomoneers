package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type HandlersCfg struct {
	Name    string
	Command string
}

type HandlersCfgs struct {
	Handlers []HandlersCfg `handlers`
}

func NewHandlersCfgs(filename string) (*HandlersCfgs, error) {
	t := &HandlersCfgs{}
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
