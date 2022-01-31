package sinks

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	_ "github.com/mattn/go-sqlite3"
)

const SQLITE3_TIMESTAMP_NAME = `timestamp`
const SQLITE3_TIMESTAMP_TYPE = `TIMESTAMP NOT NULL`

type SqliteTable struct {
	columns     []string
	coltypes    []string
	createQuery string
	insertQuery string
	primkeys    []string
}

type SqliteSink struct {
	sink
	db     *sql.DB
	tables map[string]SqliteTable
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

func (s *SqliteSink) Init(config sinkConfig) error {
	var err error
	if len(config.Database) == 0 {
		return errors.New("not all configuration variables set required by SqliteSink")
	}
	s.host = config.Host
	s.port = config.Port
	s.database = config.Database
	s.organization = config.Organization
	s.user = config.User
	s.password = config.Password
	log.Print("Opening Sqlite3 database ", s.database)
	uri := fmt.Sprintf("file:./%s.db", s.database)
	if len(s.user) > 0 && len(s.password) > 0 {
		uri += fmt.Sprintf("?_auth&_auth_user=%s&_auth_pass=%s", s.user, s.password)
	}
	s.db, err = sql.Open("sqlite3", uri)
	if err != nil {
		log.Fatal(err)
		s.db = nil
		return err
	}
	s.tables = make(map[string]SqliteTable)

	return nil
}

func getkeylist(point lp.CCMetric, include_meta bool) []string {
	keys := make([]string, 0)
	for k := range point.Tags() {
		keys = append(keys, k)
	}
	if include_meta {
		for k := range point.Meta() {
			keys = append(keys, k)
		}
	}
	for k := range point.Fields() {
		keys = append(keys, k)
	}
	keys = append(keys, SQLITE3_TIMESTAMP_NAME)
	sort.Sort(StrList(keys))
	return keys
}

func getvaluelist(point lp.CCMetric, keys []string) []string {
	values := make([]string, 0)
	for _, key := range keys {
		if key == SQLITE3_TIMESTAMP_NAME {
			values = append(values, point.Time().String())
		} else if val, ok := point.GetTag(key); ok {
			values = append(values, val)
		} else if val, ok := point.GetMeta(key); ok {
			values = append(values, val)
		} else if ival, ok := point.GetField(key); ok {
			values = append(values, fmt.Sprintf("%v", ival))
		} else {
			values = append(values, "NULL")
		}
	}
	return values
}
func gettypelist(point lp.CCMetric, keys []string) []string {
	types := make([]string, 0)
	for _, key := range keys {
		if key == SQLITE3_TIMESTAMP_NAME {
			types = append(types, SQLITE3_TIMESTAMP_TYPE)
			continue
		}
		if point.HasTag(key) {
			types = append(types, "TEXT")
			continue
		}
		if point.HasMeta(key) {
			types = append(types, "TEXT")
			continue
		}
		ival, ok := point.GetField(key)
		if ok {
			switch ival.(type) {
			case float64:
				types = append(types, "DOUBLE")
			case float32:
				types = append(types, "FLOAT")
			case string:
				types = append(types, "TEXT")
			case int:
				types = append(types, "INT")
			case int64:
				types = append(types, "INT8")
			}
		}
	}
	return types
}

func getprimkey(keys []string) []string {
	primkeys := make([]string, 0)
	primkeys = append(primkeys, SQLITE3_TIMESTAMP_NAME)
	for _, key := range keys {
		switch key {
		case "hostname":
			primkeys = append(primkeys, "hostname")
		case "type":
			primkeys = append(primkeys, "type")
		case "type-id":
			primkeys = append(primkeys, "type-id")
		}
	}
	return primkeys
}

func newCreateQuery(tablename string, keys []string, types []string, primkeys []string) string {

	keytypelist := make([]string, 0)
	for i, key := range keys {
		keytypelist = append(keytypelist, fmt.Sprintf("%s %s", key, types[i]))
	}
	keytypelist = append(keytypelist, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primkeys, ",")))
	stmt := fmt.Sprintf("create table if not exists %s (%s);", tablename, keytypelist)
	return stmt
}

func newInsertQuery(tablename string, keys []string) string {
	v := strings.Repeat("?,", len(keys)) + "?"
	stmt := fmt.Sprintf("insert into %s (%s) values(%s);", tablename, strings.Join(keys, ","), v)
	return stmt
}

func (s *SqliteSink) Write(point lp.CCMetric) error {

	if s.db != nil {
		measurement := point.Name()
		if tab, ok := s.tables[measurement]; !ok {
			var tab SqliteTable
			tab.columns = getkeylist(point, s.meta_as_tags)
			tab.coltypes = gettypelist(point, tab.columns)
			tab.primkeys = getprimkey(tab.columns)
			tab.createQuery = newCreateQuery(measurement, tab.columns, tab.coltypes, tab.primkeys)
			tab.insertQuery = newInsertQuery(measurement, tab.columns)

			tx, err := s.db.Begin()
			if err != nil {
				cclog.ComponentError("SqliteSink", "Init DB session failed:", err.Error())
				return err
			}
			_, err = tx.Exec(tab.createQuery)
			if err != nil {
				cclog.ComponentError("SqliteSink", "Execute CreateQuery failed:", err.Error())
				return err
			}
			stmt, err := tx.Prepare(tab.insertQuery)
			if err != nil {
				cclog.ComponentError("SqliteSink", "Prepare InsertQuery failed:", err.Error())
				return err
			}
			defer stmt.Close()
			_, err = stmt.Exec(getvaluelist(point, tab.columns))
			if err != nil {
				cclog.ComponentError("SqliteSink", "Execute InsertQuery failed:", err.Error())
				return err
			}
			tx.Commit()
			s.tables[measurement] = tab

		} else {

			keys := getkeylist(point, s.meta_as_tags)
			if len(keys) > len(tab.columns) {
				cclog.ComponentDebug("SqliteSink", "Metric", measurement, "has different keys as creation keys, ignoring addition keys")
			} else if len(keys) < len(tab.columns) {
				cclog.ComponentDebug("SqliteSink", "Metric", measurement, "has different keys as creation keys, setting missing values with 'NULL'")
			}
			values := getvaluelist(point, tab.columns)
			tx, err := s.db.Begin()
			if err != nil {
				cclog.ComponentError("SqliteSink", "Init DB session failed:", err.Error())
				return err
			}
			stmt, err := tx.Prepare(tab.insertQuery)
			if err != nil {
				cclog.ComponentError("SqliteSink", "Prepare InsertQuery failed:", err.Error())
				return err
			}
			defer stmt.Close()
			_, err = stmt.Exec(values)
			if err != nil {
				cclog.ComponentError("SqliteSink", "Execute InsertQuery failed:", err.Error())
				return err
			}
			tx.Commit()

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
