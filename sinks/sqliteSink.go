package sinks

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"sort"
	"strings"
	"time"
	"errors"
)

type SqliteSink struct {
	Sink
	db      *sql.DB
	columns map[string][]string
}

type StrList []string

func (list StrList) Len() int { return len(list) }

func (list StrList) Swap(i, j int) { list[i], list[j] = list[j], list[i] }

func (list StrList) Less(i, j int) bool {
	var si string = list[i]
	var sj string = list[j]
	var si_lower = strings.ToLower(si)
	var sj_lower = strings.ToLower(sj)
	if si_lower == sj_lower {
		return si < sj
	}
	return si_lower < sj_lower
}

func (s *SqliteSink) Init(config SinkConfig) error {
	var err error
	if len(config.Database) == 0 {
		return errors.New("Not all configuration variables set required by SqliteSink")
	}
	s.host = config.Host
	s.port = config.Port
	s.database = config.Database
	s.organization = config.Organization
	s.user = config.User
	s.password = config.Password
	log.Print("Opening Sqlite3 database ", s.database)
	s.db, err = sql.Open("sqlite3", fmt.Sprintf("./%s.db", s.database))
	if err != nil {
		log.Fatal(err)
		s.db = nil
		return err
	}

	return nil
}

func (s *SqliteSink) PrepareLists(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) ([]string, []string, []string) {
	keys := make([]string, 0)
	values := make([]string, 0)
	keytype := make([]string, 0)

	keys = append(keys, "time")
	values = append(values, fmt.Sprintf("%d", t.Unix()))
	keytype = append(keytype, fmt.Sprintf("time INT8"))
	for k, v := range tags {
		keys = append(keys, k)
		values = append(values, fmt.Sprintf("%q", v))
		keytype = append(keytype, fmt.Sprintf("%s TEXT", k))
	}
	for k, v := range fields {
		keys = append(keys, k)
		switch v.(type) {
		case float32:
			keytype = append(keytype, fmt.Sprintf("%s FLOAT", k))
			values = append(values, fmt.Sprintf("%f", v))
		case float64:
			keytype = append(keytype, fmt.Sprintf("%s DOUBLE", k))
			values = append(values, fmt.Sprintf("%f", v))
		case int64:
			keytype = append(keytype, fmt.Sprintf("%s INT8", k))
			values = append(values, fmt.Sprintf("%d", v))
		case int:
			keytype = append(keytype, fmt.Sprintf("%s INT", k))
			values = append(values, fmt.Sprintf("%d", v))
		case string:
			keytype = append(keytype, fmt.Sprintf("%s TEXT", k))
			values = append(values, fmt.Sprintf("%q", v))
		}
	}
	sort.Sort(StrList(keytype))
	return keytype, keys, values
}

func (s *SqliteSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
    if s.db != nil {
	    keytype, keys, values := s.PrepareLists(measurement, tags, fields, t)
	    prim_key := []string{"time", "host"}
	    if measurement == "cpu" {
		    prim_key = append(prim_key, "cpu")
	    } else if measurement == "socket" {
		    prim_key = append(prim_key, "socket")
	    }
	    tx, err := s.db.Begin()
	    if err == nil {
		    c := fmt.Sprintf("create table if not exists %s (%s, PRIMARY KEY (%s));", measurement,
			    strings.Join(keytype, ","),
			    strings.Join(prim_key, ","))
		    i := fmt.Sprintf("insert into %s (%s) values(%s);", measurement, strings.Join(keys, ","), strings.Join(values, ","))
		    _, err = tx.Exec(c)
		    if err != nil {
			    log.Println(err)
		    }
		    _, err = tx.Exec(i)
		    if err != nil {
			    log.Println(err)
		    }
		    err = tx.Commit()
		    if err != nil {
			    log.Println(err)
		    }
	    } else {
		    log.Println(err)
	    }
	}
	return nil
}

func (s *SqliteSink) Flush() error {
	return nil
}

func (s *SqliteSink) Close() {
    log.Print("Closing Sqlite3 database ", s.database)
    if s.db != nil {
    	s.db.Close()
    }
}
