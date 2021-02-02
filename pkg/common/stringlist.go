package common

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
)

type stringList []string

func (i *stringList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *stringList) String() string {
	return strings.Join(*i, ", ")
}

func (i *stringList) IsCumulative() bool {
	return true
}

func StringList(s kingpin.Settings) (target *[]string) {
	target = new([]string)
	s.SetValue((*stringList)(target))
	return
}

func InitLogging(logLevel *string) {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Errorf("Cannot init set logger level: %s", err)
		os.Exit(-1)
	}

	log.SetLevel(level)
}
