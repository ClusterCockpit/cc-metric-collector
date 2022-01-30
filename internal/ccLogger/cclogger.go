package cclogger

import (
	"fmt"
	"log"
	"os"
	"runtime"
)

var (
	globalDebug             = false
	stdout                  = os.Stdout
	stderr                  = os.Stderr
	debugLog    *log.Logger = nil
	infoLog     *log.Logger = nil
	errorLog    *log.Logger = nil
	warnLog     *log.Logger = nil
	defaultLog  *log.Logger = nil
)

func initLogger() {
	if debugLog == nil {
		debugLog = log.New(stderr, "DEBUG ", log.LstdFlags)
	}
	if infoLog == nil {
		infoLog = log.New(stdout, "INFO ", log.LstdFlags)
	}
	if errorLog == nil {
		errorLog = log.New(stderr, "ERROR ", log.LstdFlags)
	}
	if warnLog == nil {
		warnLog = log.New(stderr, "WARN ", log.LstdFlags)
	}
	if defaultLog == nil {
		defaultLog = log.New(stdout, "", log.LstdFlags)
	}
}

func Print(e ...interface{}) {
	initLogger()
	defaultLog.Print(e...)
}

func ComponentPrint(component string, e ...interface{}) {
	initLogger()
	defaultLog.Print(fmt.Sprintf("[%s] ", component), e)
}

func Info(e ...interface{}) {
	initLogger()
	infoLog.Print(e...)
}

func ComponentInfo(component string, e ...interface{}) {
	initLogger()
	infoLog.Print(fmt.Sprintf("[%s] ", component), e)
}

func Debug(e ...interface{}) {
	initLogger()
	if globalDebug {
		debugLog.Print(e...)
	}
}

func ComponentDebug(component string, e ...interface{}) {
	initLogger()
	if globalDebug && debugLog != nil {
		//CCComponentPrint(debugLog, component,  e)
		debugLog.Print(fmt.Sprintf("[%s] ", component), e)
	}
}

func Error(e ...interface{}) {
	initLogger()
	_, fn, line, _ := runtime.Caller(1)
	errorLog.Print(fmt.Sprintf("[%s:%d] ", fn, line), e)
}

func ComponentError(component string, e ...interface{}) {
	initLogger()
	_, fn, line, _ := runtime.Caller(1)
	errorLog.Print(fmt.Sprintf("[%s|%s:%d] ", component, fn, line), e)
}

func SetDebug() {
	globalDebug = true
	initLogger()
}

func SetOutput(filename string) {
	if filename == "stderr" {
		if stderr != os.Stderr && stderr != os.Stdout {
			stderr.Close()
		}
		stderr = os.Stderr
	} else if filename == "stdout" {
		if stderr != os.Stderr && stderr != os.Stdout {
			stderr.Close()
		}
		stderr = os.Stdout
	} else {
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			defer file.Close()
			stderr = file
		}
	}
	debugLog = nil
	errorLog = nil
	warnLog = nil
	initLogger()
}
