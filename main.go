package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"

	"github.com/h2non/filetype"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	chLogPath = kingpin.Arg("file", "Path to clickhouse log file").Required().String()
	fromStart = kingpin.Flag("from-start", "Read log file from start, false by default").Default("false").Bool()
	listen    = kingpin.Flag("listen", "Address to be listened by prometheus client").Default("0.0.0.0:19901").String()
)

func init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func checkLogFile(path string) error {
	log.WithFields(log.Fields{"file": path}).Info("Checking file")
	// Checking if file exists
	stat, err := os.Stat(path)
	if err != nil {
		log.WithFields(log.Fields{"file": path, "error": err}).Error("Can't stat path")
		return err
	}
	// Checking if it's a directory
	if stat.IsDir() {
		log.WithFields(log.Fields{"file": path}).Error("It's a directory")
		return err
	}

	// Checking if the file is not archive log
	file, _ := os.Open(path)
	head := make([]byte, 261)
	file.Read(head)
	if filetype.IsArchive(head) {
		log.WithFields(log.Fields{}).Error("File is an archive")
		return errors.New("File is an archive")
	}

	return nil
}

func startPrometheusListener() *http.Server {
	srv := &http.Server{Addr: *listen}

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.WithFields(log.Fields{"error": err}).Error("Can't start http listener")
			log.Exit(1)
		}
	}()

	return srv
}

func main() {
	var err error

	// Getting flags
	kingpin.Parse()
	log.WithFields(log.Fields{"file": *chLogPath}).Info("Got clickhouse log path")

	// Starting prometheus listener
	srv := startPrometheusListener()

	// Check file
	err = checkLogFile(*chLogPath)
	if err != nil {
		log.WithFields(log.Fields{"path": *chLogPath}).Error("Can't parse file")
		log.Exit(1)
	}

	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	// Starting tail
	log.WithFields(log.Fields{"path": *chLogPath}).Info("Processing file")
	tailLogger := log.WithFields(log.Fields{"type": "tail"})
	// read from the end by default
	whence := 2
	if *fromStart {
		whence = 0
		log.WithFields(log.Fields{"whence": whence}).Info("Reading from start")
	}
	seekInfo := tail.SeekInfo{Offset: 0, Whence: whence}
	tailConfig := tail.Config{Location: &seekInfo, Follow: true, ReOpen: true, MustExist: true, Poll: false, Pipe: false, Logger: tailLogger}
	t, err := tail.TailFile(*chLogPath, tailConfig)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Can't parse file with tail")
		log.Exit(1)
	}

	queries := chQueries{}
	queries.Queries = make(map[string]*chQuery)
	for {
		select {
		case line := <-t.Lines:
			if line.Err != nil {
				chLogExporterErrors.WithLabelValues("tail_line").Inc()
				log.WithFields(log.Fields{}).Warn("Got error from tail")
				continue
			}

			readLines.Inc()
			query := queries.ProcessQuery(line.Text)
			if query == nil {
				continue
			}

			if query.FullInfo {
				// TODO: Send info about query
				log.WithFields(log.Fields{"id": query.ID}).Debug("Got full info about query")
				delete(queries.Queries, query.ID)
			}
		case <-signals:
			log.Printf("Got system signal, exiting...\n")

			// removing inotify from file
			t.Cleanup()

			// Stopping prometheus listener
			if err = srv.Shutdown(context.TODO()); err != nil {
				log.WithFields(log.Fields{"error": err}).Error("Can't stop prometheus listener")
				log.Exit(1)
			}

			return
		}
	}
}
