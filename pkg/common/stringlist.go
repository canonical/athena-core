package common

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
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

type AthenaFormatter struct{}

func (f *AthenaFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s [%s]: %s\n", entry.Time.Format("2006-01-02 15:04:05"), entry.Level.String(), entry.Message)), nil
}

func ParseCommandline() {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
}

func InitLogging(logLevel *string) {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&AthenaFormatter{})

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
