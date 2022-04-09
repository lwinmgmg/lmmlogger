package lmmlogger

import (
	"fmt"
	"os"
	"time"
)

type Logger interface {
	Info(string, ...string)
	Error(string, ...string)
	Warning(string, ...string)
	Close()
}

type Log struct {
	Chan     chan []string
	Done     chan struct{}
	Filename string
	Format   string
}

func NewLogger(filaName string, fileOnly bool, consoleOnly bool, size int) *Log {
	ch := make(chan []string, size)
	done := make(chan struct{})
	// file, err := os.OpenFile(filaName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	// if err != nil {
	// 	fmt.Println("Can't Open file")
	// 	return nil
	// }
	go func(ch <-chan []string, doneCh <-chan struct{}) {
		var mesg []string
		for {
			select {
			case mesg = <-ch:
				if !fileOnly {
					fmt.Println(mesg)
				}
				if !consoleOnly {
					// if err = writeToAFile(file, mesg); err != nil {
					// 	fmt.Println("Can't write to log file")
					// }
				}
			case <-done:
				return
			}
		}
	}(ch, done)
	return &Log{
		Chan:     ch,
		Done:     done,
		Filename: filaName,
		Format:   "abcd",
	}
}

func writeToAFile(file *os.File, mesg string) error {
	if _, err := file.Write([]byte(fmt.Sprintln(mesg))); err != nil {
		return err
	}
	return nil
}

func convertToLogFormat(mesg string, level string, multiStr []string) string {
	tempMessage := fmt.Sprintf("%v  %-7s  %v", time.Now().Format(time.RFC1123), level, mesg)
	tmp := make([]interface{}, len(multiStr))
	for i, val := range multiStr {
		tmp[i] = val
	}
	return fmt.Sprintf(tempMessage, tmp...)
}

func (self Log) Info(firstStr string, multiStr ...string) {
	self.Chan <- append([]string{firstStr}, multiStr...)
}
func (self Log) Error(firstStr string, multiStr ...string) {
	self.Chan <- append([]string{firstStr}, multiStr...)
}
func (self Log) Warning(firstStr string, multiStr ...string) {
	self.Chan <- append([]string{firstStr}, multiStr...)
}
func (self Log) Close() {
	close(self.Done)
}

func searcher(sec int) string {
	secInMilli := 1000 * sec
	time.Sleep(time.Millisecond * time.Duration(secInMilli))
	return fmt.Sprintf("Time in %v", sec)
}
