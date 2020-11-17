package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
)

func init() {
	// Register an http server with a random port to pprof
	go func() { _ = http.ListenAndServe(":0", nil) }()
}

func main() {
	var logLevel, listenAddr, remoteWriteAddr string
	flag.StringVar(&logLevel, "logLevel", "info", "log level")
	flag.StringVar(&listenAddr, "listenAddr", ":2004", "listen address")
	flag.StringVar(&remoteWriteAddr, "remoteWriteAddr", "127.0.0.1:2003", "VictoriaMetrics graphite listen address https://github.com/VictoriaMetrics/VictoriaMetrics#how-to-send-data-from-graphite-compatible-agents-such-as-statsd")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	readerPool := &sync.Pool{
		New: func() interface{} {
			return bufio.NewReaderSize(nil, 64*1024)
		},
	}
	writerPool := &sync.Pool{
		New: func() interface{} {
			return bufio.NewWriterSize(nil, 64*1024)
		},
	}
	builderPool := &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 1024))
		},
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("accept failed %s", err)
			continue
		}

		go func(localConn net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("catch panic %s", r)
				}
			}()

			remoteConn, err := net.Dial("tcp", remoteWriteAddr)
			if err != nil {
				log.Errorf("dial failed %s", err)
				return
			}

			defer func() { _ = localConn.Close() }()
			defer func() { _ = remoteConn.Close() }()

			reader := readerPool.Get().(*bufio.Reader)
			defer readerPool.Put(reader)
			reader.Reset(localConn)

			writer := writerPool.Get().(*bufio.Writer)
			defer writerPool.Put(writer)
			writer.Reset(remoteConn)

			defer func() { _ = writer.Flush() }()

			builder := builderPool.Get().(*bytes.Buffer)
			defer builderPool.Put(builder)

			var next []byte
			for {
				line, isContinue, err := reader.ReadLine()
				for isContinue && err == nil {
					next, isContinue, err = reader.ReadLine()
					line = append(line, next...)
				}
				// carbon-c-relay closes the tcp connection directly after sending.
				// So io.EOF errors mean that it is closed properly.
				if err == io.EOF {
					return
				}

				if err != nil {
					log.Errorf("read from %s failed %s", localConn.RemoteAddr(), err)
					return
				}

				builder.Reset()
				success := convertGraphite(builder, line)
				if !success {
					log.Debugf("ignore invalid metric %s", line)
					continue
				}

				_, err = writer.Write(builder.Bytes())
				if err != nil {
					log.Errorf("write to %s failed %s", remoteConn.RemoteAddr(), err)
					return
				}

				if writer.Available() < 8192 {
					err = writer.Flush()
				}
				if err != nil {
					log.Errorf("write to %s failed %s", remoteConn.RemoteAddr(), err)
					return
				}
			}
		}(conn)
	}
}

func convertGraphite(builder *bytes.Buffer, line []byte) bool {
	i1 := bytes.IndexByte(line, ' ')
	if i1 < 0 {
		return false
	}
	i2 := bytes.IndexByte(line[i1+1:], ' ')
	if i2 < 0 {
		return false
	}
	i3 := bytes.IndexByte(line[i1+1+i2+1:], ' ')
	if i3 > 0 {
		return false
	}

	metricName := line[:i1]
	metricValue := line[i1+1 : i1+1+i2]
	metricTime := line[i1+1+i2+1:]

	labels := bytes.Split(metricName, []byte("."))
	if len(labels) < 2 {
		return false
	}

	// We will try to put the first segment of graphite in the label that follows, and the `-` is not allowed in the Label.
	var prefixLabel []byte
	if !bytes.ContainsRune(labels[0], '-') {
		prefixLabel = labels[0]
	} else {
		prefixLabel = bytes.ReplaceAll(labels[0], []byte("-"), []byte("_"))
	}
	builder.Write(prefixLabel)

	for i := 1; i < len(labels); i++ {
		builder.Write([]byte(";__"))
		builder.Write(prefixLabel)
		builder.Write([]byte("_g"))
		builder.WriteString(strconv.Itoa(i))
		builder.Write([]byte("__="))
		builder.Write(labels[i])
	}

	builder.WriteByte(' ')
	builder.Write(metricValue)
	builder.WriteByte(' ')
	builder.Write(metricTime)
	builder.WriteByte('\n')

	return true
}
