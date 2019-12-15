package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	log "github.com/sirupsen/logrus"
	"github.com/xwb1989/sqlparser"
)

var (
	initialQueryRe   = regexp.MustCompile(`^(?P<dt>\d{1,4}[.\-/]\d{1,2}[.\-/]\d{1,4} \d{1,2}\:\d{1,2}\:\d{1,2}\.\d{1,6}) \[ (?P<pid>\d+) \] \{(?P<id>.*)\} <Debug> executeQuery: \(from (?P<host>(?:[0-9]{1,3}\.){3}[0-9]{1,3}):(?P<port>\d+)(?P<additional>.*?)\) (?P<query>.*)$`)
	processedQueryRe = regexp.MustCompile(`^(?P<dt>\d{1,4}[.\-/]\d{1,2}[.\-/]\d{1,4} \d{1,2}\:\d{1,2}\:\d{1,2}\.\d{1,6}) \[ (?P<pid>\d+) \] \{(?P<id>.*)\} <Information> executeQuery: Read (?P<rows>\d+) rows, (?P<bytes>[.\d]+ \w+) in (?P<elapsed>[.\d]+) sec., (?P<rps>[.\d]+) rows/sec., (?P<speed>[.\d]+ \w+)[/\w]+.$`)
	memoryQueryRe    = regexp.MustCompile(`^(?P<dt>\d{1,4}[.\-/]\d{1,2}[.\-/]\d{1,4} \d{1,2}\:\d{1,2}\:\d{1,2}\.\d{1,6}) \[ (?P<pid>\d+) \] \{(?P<id>.*)\} <Debug> MemoryTracker: Peak memory usage \(for query\): (?P<bytes>[.\d]+ \w+).$`)
	errorQueryRe     = regexp.MustCompile(`^(?P<dt>\d{1,4}[.\-/]\d{1,2}[.\-/]\d{1,4} \d{1,2}\:\d{1,2}\:\d{1,2}\.\d{1,6}) \[ (?P<pid>\d+) \] \{(?P<id>.*)\} <Error> executeQuery: Code: (?P<code>\d+), e\.displayText\(\) = (?P<message>.*)$`)
)

type chQueries struct {
	Queries map[string]*chQuery
}

type chQuery struct {
	ID           string    `json:"id"`
	Host         string    `json:"host"`
	Port         int64     `json:"port"`
	Pid          int64     `json:"pid"`
	Query        string    `json:"query"`
	StartTime    time.Time `json:"starttime"`
	EndTime      time.Time `json:"endtime"`
	RowsRead     int64     `json:"rowsread"`
	BytesRead    uint64    `json:"bytesread"`
	Elapsed      float64   `json:"elapsed"`
	Rps          int64     `json:"rps"`     // rows per sec
	Speed        uint64    `json:"bps"`     // bytes / sec
	Memory       uint64    `json:"memused"` // bytes
	FullInfo     bool      `json:"-"`
	Error        bool      `json:"err"`
	ErrorCode    int64     `json:"errcode"`
	ErrorMessage string    `json:"errmsg"`
	StmtType     int       `json:"stmttype"`
	TCPProcessed float64   `json:"tcpprocessed"`
}

func (c *chQueries) ProcessQuery(line string) *chQuery {
	// Skip everything except information and debug and error
	if !strings.Contains(line, "Debug") && !strings.Contains(line, "Information") && !strings.Contains(line, "Error") {
		return nil
	}

	// Checking initial query
	query := c.processInitialQuery(line)
	if query != nil {
		return query
	}

	// Checking stats query
	query = c.processStatsQuery(line)
	if query != nil {
		return query
	}

	// Checking error query
	query = c.processErrorQuery(line)
	if query != nil {
		return query
	}

	// Checking memory query
	query = c.processMemoryQuery(line)
	if query != nil {
		return query
	}

	return nil

}

func (c *chQueries) processInitialQuery(line string) *chQuery {
	match := initialQueryRe.FindStringSubmatch(line)
	if len(match) > 0 {
		data := make(map[string]string)
		for i, name := range initialQueryRe.SubexpNames() {
			if i != 0 && name != "" {
				data[name] = match[i]
			}
		}

		query, ok := c.Queries[data["id"]]
		if ok {
			chLogExporterErrors.WithLabelValues("duplicated_initial_query").Inc()
			log.WithFields(log.Fields{"id": data["id"], "type": "initial"}).Warn("Duplicated queries in log found")
		} else {
			pid, err := strconv.ParseInt(data["pid"], 10, 64)
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"pid": data["pid"]}).Error("Can't convert pid to int")
				return nil
			}

			port, err := strconv.ParseInt(data["port"], 10, 64)
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"port": data["port"]}).Error("Can't convert port to int")
				return nil
			}

			layout := "2006.01.02 15:04:05.999999"
			dt, err := time.Parse(layout, data["dt"])
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"date": data["dt"]}).Error("Can't convert datetime to time")
				return nil
			}

			query = &chQuery{}
			query.ID = data["id"]
			query.StartTime = dt
			query.Host = data["host"]
			query.Pid = pid
			query.Port = port
			query.Query = data["query"]
			query.StmtType = sqlparser.Preview(query.Query)

			chQueryCount.WithLabelValues(getStmtType(query.StmtType)).Inc()

			c.Queries[data["id"]] = query
		}

		return query
	}
	return nil
}

func (c *chQueries) processStatsQuery(line string) *chQuery {
	match := processedQueryRe.FindStringSubmatch(line)
	if len(match) > 0 {
		data := make(map[string]string)
		for i, name := range processedQueryRe.SubexpNames() {
			if i != 0 && name != "" {
				data[name] = match[i]
			}
		}

		query, ok := c.Queries[data["id"]]
		if ok {
			// Converting values
			bytesRead, err := bytefmt.ToBytes(strings.Join(strings.Fields(data["bytes"]), ""))
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"bytes": data["bytes"]}).Error("Can't convert read bytes")
				return nil
			}

			speed, err := bytefmt.ToBytes(strings.Join(strings.Fields(data["speed"]), ""))
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"speed": data["speed"]}).Error("Can't convert speed to bytes/sec")
				return nil
			}

			rowsRead, err := strconv.ParseInt(data["rows"], 10, 64)
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"rows": data["rows"]}).Error("Can't convert rows to int")
				return nil
			}

			rps, err := strconv.ParseInt(data["rps"], 10, 64)
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"rps": data["rps"]}).Error("Can't convert rps to int")
				return nil
			}

			query.RowsRead = rowsRead
			query.BytesRead = bytesRead
			query.Rps = rps
			query.Speed = speed

			chSelectQueryRowsRead.Observe(float64(query.RowsRead))
			chSelectQueryBytesRead.Observe(float64(query.BytesRead))
			chSelectQueryRowsPerSecond.Observe(float64(query.Rps))
			chSelectQueryBytesPerSecond.Observe(float64(query.Speed))

			return query
		}
		chLogExporterErrors.WithLabelValues("not_found_query").Inc()
		log.WithFields(log.Fields{"id": data["id"], "type": "processed"}).Warn("Can't find such query, might be in another log file")
	}

	return nil
}

func (c *chQueries) processMemoryQuery(line string) *chQuery {
	match := memoryQueryRe.FindStringSubmatch(line)
	if len(match) > 0 {
		data := make(map[string]string)
		for i, name := range memoryQueryRe.SubexpNames() {
			if i != 0 && name != "" {
				data[name] = match[i]
			}
		}

		query, ok := c.Queries[data["id"]]
		if ok {
			layout := "2006.01.02 15:04:05.999999"
			dt, err := time.Parse(layout, data["dt"])
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"date": data["dt"]}).Error("Can't convert datetime to time")
				return nil
			}

			memoryUsed, err := bytefmt.ToBytes(strings.Join(strings.Fields(data["bytes"]), ""))
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"bytes": data["bytes"]}).Error("Can't convert read bytes")
				return nil
			}

			query.Memory = memoryUsed
			query.EndTime = dt
			query.Elapsed = dt.Sub(query.StartTime).Seconds()

			chQueryTime.WithLabelValues(getStmtType(query.StmtType)).Observe(query.Elapsed)

			// Finish getting info for !insert queries
			if query.StmtType != sqlparser.StmtInsert {
				query.FullInfo = true
			}

			return query
		}
		chLogExporterErrors.WithLabelValues("not_found_query").Inc()
		log.WithFields(log.Fields{"id": data["id"], "type": "memory"}).Warn("Can't find such query, might be in another log file")
	}
	return nil
}

func (c *chQueries) processErrorQuery(line string) *chQuery {
	match := errorQueryRe.FindStringSubmatch(line)
	if len(match) > 0 {
		data := make(map[string]string)
		for i, name := range errorQueryRe.SubexpNames() {
			if i != 0 && name != "" {
				data[name] = match[i]
			}
		}

		query, ok := c.Queries[data["id"]]
		if ok {
			code, err := strconv.ParseInt(data["code"], 10, 16)
			if err != nil {
				chLogExporterErrors.WithLabelValues("convert").Inc()
				log.WithFields(log.Fields{"code": data["code"]}).Error("Can't convert code to int")
				return nil
			}

			chQueryErrors.WithLabelValues(getStmtType(query.StmtType), strconv.FormatInt(code, 10)).Inc()

			query.Error = true
			query.ErrorCode = code
			query.ErrorMessage = data["message"]

			return query
		}
		chLogExporterErrors.WithLabelValues("not_found_query").Inc()
		log.WithFields(log.Fields{"id": data["id"], "type": "error"}).Warn("Can't find such query, might be in another log file")
	}
	return nil
}

func getStmtType(stmtType int) string {
	switch stmtType {
	case sqlparser.StmtSelect:
		return "select"
	case sqlparser.StmtInsert:
		return "insert"
	case sqlparser.StmtUpdate:
		return "update"
	case sqlparser.StmtDelete:
		return "delete"
	default:
		return "other"
	}
}
