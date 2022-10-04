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
	// TODO: add if anything is configurable.
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

	var (
		cfg     config
		session sessionData
	)

	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("Couldn't read configuration file %s: %v", configFile, err)
	} else {
		if err := yaml.Unmarshal(configData, &cfg); err != nil {
			log.Printf("Couldn't unmarshal configuration file %s: %v", configFile, err)
		} else {
			// TODO: set settings
		}
	}

	sessionFileData, err := ioutil.ReadFile(sessionFile)
	if err != nil {
		log.Printf("Couldn't read session file: %s: %v", configFile, err)
	} else {
		if err := yaml.Unmarshal(sessionFileData, &session); err != nil {
			log.Printf("Couldn't unmarshal session file %s: %v", sessionFile, err)
		} else {
			ctrl.restoreSession(session)
		}
	}

	if err := view.run(); err != nil {
		fail("Starting koios review failed: %v", err)
	}

	sessionFileData, err = yaml.Marshal(ctrl.getSession())
	if err != nil {
		fail("Marshalling session data failed: %v", err)
	}

	if err := ioutil.WriteFile(sessionFile, sessionFileData, 0600); err != nil {
		fail("Saving session data failed: %v", err)
	}
}
