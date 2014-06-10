package udp

import (
	"fmt"

	"github.com/reusee/closer"
	ic "github.com/reusee/inf-chan"
)

type Logger struct {
	closer.Closer
	logsIn chan string
	Logs   chan string
}

func newLogger() *Logger {
	logger := &Logger{
		logsIn: make(chan string),
		Logs:   make(chan string),
	}
	ic.Link(logger.logsIn, logger.Logs)
	logger.OnClose(func() {
		close(logger.logsIn)
	})
	return logger
}

func (l *Logger) Log(format string, args ...interface{}) {
	l.logsIn <- fmt.Sprintf(format, args...)
}
