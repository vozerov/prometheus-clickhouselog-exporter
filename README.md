Prometheus Clickhouse Log Exporter
===========

Simple tool to parse clickhouse log and obtain information about queries, errors and duration.

Build
=====
Just clone this repo and run:

```
go get
go build -o prometheus-clickhouselog-exporter *.go
```

Options
=======
Current options are:
```
usage: prometheus-clickhouselog-exporter [<flags>] <file>

Flags:
  --help                    Show context-sensitive help (also try --help-long and --help-man).
  --from-start              Read log file from start, false by default
  --listen="0.0.0.0:19901"  Address to be listened by prometheus client

Args:
  <file>  Path to clickhouse log file
  ```

Run
===
Clickhouselog-exporter will follow the file (like tail -f) and will also reopen it if it was rotated. To run it just use:
```
./prometheus-clickhouselog-exporter /var/log/clickhouse-server/clickhouse-server.log
```

Docker
======
Simply run:
```
docker run -d --name prometheus-clickhouselog-exporter -v /var/log/clickhouse-server:/var/log/clickhouse-server:ro -p 19901:19901 vozerov/prometheus-clickhouselog-exporter /var/log/clickhouse-server/clickhouse-server.log
```

TODO
====
- Parse and normalize query (remove variables)
- Post processed queries to kafka to further analysis
- Add time spent on tcp / http handling


