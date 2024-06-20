package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/syslog"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

var AvailableSyslogPriorities map[string]syslog.Priority = map[string]syslog.Priority{
	"LOG_EMERG":    syslog.LOG_EMERG,
	"LOG_ALERT":    syslog.LOG_ALERT,
	"LOG_CRIT":     syslog.LOG_CRIT,
	"LOG_ERR":      syslog.LOG_ERR,
	"LOG_NOTICE":   syslog.LOG_NOTICE,
	"LOG_INFO":     syslog.LOG_INFO,
	"LOG_DEBUG":    syslog.LOG_DEBUG,
	"LOG_USER":     syslog.LOG_USER,
	"LOG_MAIL":     syslog.LOG_MAIL,
	"LOG_DAEMON":   syslog.LOG_DAEMON,
	"LOG_AUTH":     syslog.LOG_AUTH,
	"LOG_SYSLOG":   syslog.LOG_SYSLOG,
	"LOG_LPR":      syslog.LOG_LPR,
	"LOG_NEWS":     syslog.LOG_NEWS,
	"LOG_UUCP":     syslog.LOG_UUCP,
	"LOG_CRON":     syslog.LOG_CRON,
	"LOG_AUTHPRIV": syslog.LOG_AUTHPRIV,
	"LOG_FTP":      syslog.LOG_FTP,
	"LOG_LOCAL0":   syslog.LOG_LOCAL0,
	"LOG_LOCAL1":   syslog.LOG_LOCAL1,
	"LOG_LOCAL2":   syslog.LOG_LOCAL2,
	"LOG_LOCAL3":   syslog.LOG_LOCAL3,
	"LOG_LOCAL4":   syslog.LOG_LOCAL4,
	"LOG_LOCAL5":   syslog.LOG_LOCAL5,
	"LOG_LOCAL6":   syslog.LOG_LOCAL6,
	"LOG_LOCAL7":   syslog.LOG_LOCAL7,
}

type SyslogSinkConfig struct {
	defaultSinkConfig
	SyslogTag      string   `json:"syslog_tag"`
	SyslogPriories []string `json:"syslog_priorities"`
	SyslogWriter   string   `json:"syslog_writer"`
}

type SyslogSink struct {
	sink
	config      SyslogSinkConfig // entry point to the SyslogSinkConfig
	syslogPrios syslog.Priority
	out         *syslog.Writer
}

func (s *SyslogSink) Write(point lp.CCMetric) error {
	var err error = nil
	p := point.ToLineProtocol(s.meta_as_tags)
	switch s.config.SyslogWriter {
	case "info":
		err = s.out.Info(p)
	case "debug":
		err = s.out.Debug(p)
	case "alert":
		err = s.out.Alert(p)
	case "emerg":
		err = s.out.Emerg(p)
	case "err":
		err = s.out.Err(p)
	case "notice":
		err = s.out.Notice(p)
	case "warning":
		err = s.out.Warning(p)
	}

	return err
}

func (s *SyslogSink) Flush() error {
	return nil
}

func (s *SyslogSink) Close() {
	s.out.Close()
	cclog.ComponentDebug(s.name, "CLOSE")
}

func NewSyslogSink(name string, config json.RawMessage) (Sink, error) {
	var err error = nil
	s := new(SyslogSink)

	s.name = fmt.Sprintf("SyslogSink(%s)", name)

	// Read in the config JSON
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	if len(s.config.SyslogTag) == 0 {
		err := errors.New("syslog tag must be non-empty")
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}

	if len(s.config.SyslogPriories) == 0 {
		err := errors.New("syslog prioritiey must be non-empty")
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}
	for _, p := range s.config.SyslogPriories {
		if prio, ok := AvailableSyslogPriorities[p]; !ok {
			err := fmt.Errorf("invalid syslog priority '%s'", p)
			cclog.ComponentError(s.name, err.Error())
			return nil, err
		} else {
			s.syslogPrios |= prio
		}
	}

	mywriter := ""
	for _, w := range []string{
		"info",
		"debug",
		"alert",
		"emerg",
		"err",
		"notice",
		"warning",
	} {
		if s.config.SyslogWriter == w {
			mywriter = w
		}
	}
	if len(mywriter) == 0 {
		err := fmt.Errorf("invalid syslog writer '%s'", s.config.SyslogWriter)
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}

	s.out, err = syslog.New(s.syslogPrios, s.config.SyslogTag)
	if err != nil {
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}

	return s, nil
}
