package main

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type sessionData struct {
	Databases []sessionDataDB `yaml:"databases"`
	Queries   *queriesData    `yaml:"queries"`
}

type sessionDataDB struct {
	Driver        string            `yaml:"driver"`
	ConnectParams map[string]string `yaml:"connect_params"`
}

type queriesData struct {
	Tabs  []string `yaml:"tabs"`
	Index int      `yaml:"index"`
}

func loadSession(filename string) (*sessionData, error) {
	sessionFileData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("couldn't read session file %s: %w", filename, err)
	}

	session := &sessionData{}

	if err := yaml.Unmarshal(sessionFileData, session); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal session file %s: %w", filename, err)
	}

	return session, nil
}

func storeSession(filename string, session *sessionData) error {
	sessionFileData, err := yaml.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshalling session data failed: %w", err)
	}

	if err := ioutil.WriteFile(filename, sessionFileData, 0600); err != nil {
		return fmt.Errorf("storing session data to %s failed: %w", filename, err)
	}

	return nil
}
