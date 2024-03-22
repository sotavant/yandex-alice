package main

import (
	"flag"
	"os"
)

var flagRunAddr string
var flagLogLevel string
var flagDatabaseURI string

func parseFlags() {
	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port")
	flag.StringVar(&flagLogLevel, "l", "debug", "log level")
	flag.StringVar(&flagDatabaseURI, "d", "", "database URI")
	flag.Parse()

	if envRunAddr := os.Getenv("RUN_ADDR"); envRunAddr != "" {
		flagRunAddr = envRunAddr
	}

	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		flagLogLevel = envLogLevel
	}

	if envDatabaseUri := os.Getenv("DATABASE_URI"); envDatabaseUri != "" {
		flagDatabaseURI = envDatabaseUri
	}
}
