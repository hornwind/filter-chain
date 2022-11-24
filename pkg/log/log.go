package log

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	log.SetOutput(os.Stdout)

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)
}
