package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

func fail(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

type config struct {
	Keys []struct {
		Key       string `yaml:"key"`
		Operation string `yaml:"operation"`
	} `yaml:"keys"`
}

func loadConfig(filename string) (config, error) {
	var cfg config

	configData, err := ioutil.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("couldn't read configuration file %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(configData, cfg); err != nil {
		return cfg, fmt.Errorf("couldn't unmarshal configuration file %s: %w", filename, err)
	}

	return cfg, nil
}

func main() {
	var (
		debugLogFile string
		configFile   string
		sessionFile  string
	)

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "koios")

	flag.StringVar(&debugLogFile, "debuglog", "", "debug log file")
	flag.StringVar(&configFile, "configfile", filepath.Join(configDir, "config.yml"), "configuration file")
	flag.StringVar(&sessionFile, "statefile", filepath.Join(configDir, "session.yml"), "session file")
	flag.Parse()

	if debugLogFile != "" {
		f, err := os.OpenFile(debugLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			fail("Couldn't open debug log file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Printf("Couldn't create config directory %s: %v", configDir, err)
	}

	model := newModel()
	view := newMainView()
	ctrl := newController(model, view)
	view.setController(ctrl)
	model.setController(ctrl)

	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Printf("Loading configuration failed: %v", err)
	}

	if err := view.configure(cfg); err != nil {
		fail("Configuration failed: %v", err)
	}

	session, err := loadSession(sessionFile)
	if err != nil {
		log.Printf("Loading session data failed: %v", err)
	} else {
		ctrl.restoreSession(session)
	}

	if err := view.run(); err != nil {
		fail("Starting koios review failed: %v", err)
	}

	if err := storeSession(sessionFile, ctrl.getSession()); err != nil {
		fail("Storing session data failed: %v", err)
	}
}
