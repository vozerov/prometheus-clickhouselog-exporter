FROM golang:latest AS build

WORKDIR /go/src/app
COPY *.go /go/src/app/

ENV CGO_ENABLED=0
RUN go get && go build -o prometheus-clickhouselog-exporter *.go

FROM alpine:latest

WORKDIR /app
COPY --from=build /go/src/app/prometheus-clickhouselog-exporter /app/

ENTRYPOINT ["/app/prometheus-clickhouselog-exporter"]
