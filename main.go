package lmmlogger

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	DefaultFormat        string = "%v"
	DefaultFileSize      int64  = 10 * 1024 * 1024 // 10MB log file size default
	DefaultRotationCount int    = 1
)

var (
	LogLevelPriority map[string]int8 = map[string]int8{
		"DEBUG":    1,
		"INFO":     2,
		"WARNING":  3,
		"ERROR":    4,
		"CRITICAL": 5,
	}
)

func DefaultFormatFunction() interface{} {
	return time.Now().Format(time.RFC1123Z)
}

func MoveFileRotation(fileName string, count int) (*os.File, error) {
	for i := count; i > 0; i-- {
		newFileName := fmt.Sprintf("%v%v", fileName, i)
		oldFileName := fileName
		if i-1 != 0 {
			oldFileName = fmt.Sprintf("%v%v", fileName, i-1)
		}
		err := os.Rename(oldFileName, newFileName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Error On File Move : %w", err)
		}
	}
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("Error On Creating New Log File : %w", err)
	}
	return file, nil
}

type Logger interface {
	Debug(...interface{})
	Info(...interface{})
	Error(...interface{})
	Warning(...interface{})
	Critical(...interface{})
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
	Warningf(string, ...interface{})
	Criticalf(string, ...interface{})
	Close()
}

type LogRotationOptions struct {
	MaxSize        int64
	NumberOfBackup int
}

func (self *LogRotationOptions) SetDefaultValue() {
	if self.MaxSize < 1 {
		self.MaxSize = DefaultFileSize
	}
	if self.NumberOfBackup < 1 {
		self.NumberOfBackup = DefaultRotationCount
	}
}

func (self LogRotationOptions) IsNil() bool {
	return self.MaxSize == 0 && self.NumberOfBackup == 0
}

func (self LogRotationOptions) DoFileRotation(oldFile *os.File, logger *Log) *os.File {
	fileInfo, err := oldFile.Stat()
	if err != nil {
		logger.Error("Error On Rotating Log File On File Stat")
	}
	if fileInfo.Size() > self.MaxSize {
		RemoveFileName := fmt.Sprintf("%v%v", oldFile.Name(), self.NumberOfBackup)
		err = os.Remove(RemoveFileName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Errorf("%v", err)
			return oldFile
		}
		newFile, err := MoveFileRotation(oldFile.Name(), self.NumberOfBackup)
		if err != nil {
			logger.Errorf("%v", err)
			return oldFile
		}
		return newFile
	}
	return oldFile
}

type Log struct {
	Chan               chan Message
	Done               chan struct{}
	WaitForPendingDone bool
	CurrentFile        *os.File
}

type Param struct {
	FileName           string
	LogRotationOptions LogRotationOptions
	LogLevel           string
	ConsoleOnly        bool
	PendingSize        int
	WaitForPendingDone bool
	MaxThread          int8
	Format             string
	FormatFunctions    []func() interface{}
}

func NewLogger(params Param) (*Log, error) {
	if params.PendingSize == 0 {
		params.PendingSize = 100 // 100 panding logs are allow; Who will write 100 log in one second?
	}
	if params.MaxThread == 0 {
		params.MaxThread = 1
	}
	if params.FileName == "" {
		params.ConsoleOnly = true
	}
	if !params.LogRotationOptions.IsNil() {
		params.LogRotationOptions.SetDefaultValue()
	}
	if params.Format == "" {
		params.Format = DefaultFormat
		params.FormatFunctions = []func() interface{}{DefaultFormatFunction}
	}
	ch := make(chan Message, params.PendingSize)
	var DefaultLevel int8 = 2 // INFO
	if val, ok := DefaultLevel[params.LogLevel]; ok {
		DefaultLevel = val
	}
	done := make(chan struct{})
	OutputLog := Log{
		Chan:               ch,
		Done:               done,
		WaitForPendingDone: params.WaitForPendingDone,
	}
	funcLen := len(params.FormatFunctions)
	var file *os.File
	var err error
	if !params.ConsoleOnly {
		file, err = os.OpenFile(params.FileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		OutputLog.CurrentFile = file
	}
	if err != nil {
		OutputLog.Errorf("Error On Creating Log File : %v", err)
		params.ConsoleOnly = true
		return &OutputLog, fmt.Errorf("Error On Creating Log File : %w", err) // Returned error but you can still use console logger
	}
	go func(ch <-chan Message, doneCh <-chan struct{}) {
		for {
			select {
			case mesg := <-ch:
				if mesg.Priority < DefaultLevel {
					continue
				}
				formatInterface := make([]interface{}, funcLen, funcLen)
				for k, v := range params.FormatFunctions {
					formatInterface[k] = v()
				}
				originalFormat := fmt.Sprintf(params.Format, formatInterface...)
				newFormat := fmt.Sprintf("%v %v %v\n", originalFormat, mesg.Level, mesg.Format)
				mesgStr := fmt.Sprintf(newFormat, mesg.Params...)
				if params.ConsoleOnly {
					fmt.Print(mesgStr)
				} else {
					file.Write([]byte(mesgStr))
					if !params.LogRotationOptions.IsNil() {
						file = params.LogRotationOptions.DoFileRotation(file, &OutputLog)
					}
				}
			case <-done:
				return
			}
		}
	}(ch, done)
	return &OutputLog, nil
}

type Message struct {
	Format   string
	Params   []interface{}
	Level    string
	Priority int8
}

func writeToAFile(file *os.File, mesg string) error {
	if _, err := file.Write([]byte(fmt.Sprintln(mesg))); err != nil {
		return err
	}
	return nil
}

func (self Log) Debug(input ...interface{}) {
	self.Chan <- Message{Format: fmt.Sprint(input...), Level: "  DEBUG ", Priority: LogLevelPriority["DEBUG"]}
}
func (self Log) Debugf(format string, params ...interface{}) {
	self.Chan <- Message{Format: format, Params: params, Level: "  DEBUG ", Priority: LogLevelPriority["DEBUG"]}
}

func (self Log) Info(input ...interface{}) {
	self.Chan <- Message{Format: fmt.Sprint(input...), Level: "  INFO  ", Priority: LogLevelPriority["INFO"]}
}
func (self Log) Infof(format string, params ...interface{}) {
	self.Chan <- Message{Format: format, Params: params, Level: "  INFO  ", Priority: LogLevelPriority["INFO"]}
}

func (self Log) Warningf(format string, params ...interface{}) {
	self.Chan <- Message{Format: format, Params: params, Level: " WARNING", Priority: LogLevelPriority["WARNING"]}
}
func (self Log) Warning(input ...interface{}) {
	self.Chan <- Message{Format: fmt.Sprint(input...), Level: " WARNING", Priority: LogLevelPriority["WARNING"]}
}

func (self Log) Error(input ...interface{}) {
	self.Chan <- Message{Format: fmt.Sprint(input...), Level: "  ERROR ", Priority: LogLevelPriority["ERROR"]}
}
func (self Log) Errorf(format string, params ...interface{}) {
	self.Chan <- Message{Format: format, Params: params, Level: "  ERROR ", Priority: LogLevelPriority["ERROR"]}
}

func (self Log) Criticalf(format string, params ...interface{}) {
	self.Chan <- Message{Format: format, Params: params, Level: "CRITICAL", Priority: LogLevelPriority["CRITICAL"]}
}
func (self Log) Critical(input ...interface{}) {
	self.Chan <- Message{Format: fmt.Sprint(input...), Level: "CRITICAL", Priority: LogLevelPriority["CRITICAL"]}
}

func (self Log) Close() {
	for {
		if self.WaitForPendingDone {
			if len(self.Chan) == 0 {
				self.Done <- struct{}{}
				return
			} else {
				time.Sleep(time.Millisecond * 100)
			}
		} else {
			break
		}
	}
	self.Done <- struct{}{}
}
