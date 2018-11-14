package glbgelf

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// Logger gelf logger
	Logger *Gelf
)

// Gelf struct
type Gelf struct {
	writer      interface{}
	appName     string
	tags        string
	hostname    string
	development bool
	protocol    string
}

// 1k bytes buffer by default
var bufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

func newBuffer() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	if b != nil {
		b.Reset()
		return b
	}
	return bytes.NewBuffer(nil)
}

// SendLog send log to graylog
func (g *Gelf) SendLog(extra map[string]interface{}, loglevel string, messages ...interface{}) error {

	levels := map[string]int32{"EMERG": 0, "ALERT": 1, "CRIT": 2, "ERROR": 3, "WARNING": 4, "NOTICE": 5, "INFO": 6, "DEBUG": 7}

	if _, ok := levels[loglevel]; !ok {
		log.Println("Invalid loglevel!")
		return errors.New("Invalid loglevel!")
	}

	strmessage := make([]string, len(messages))
	for k, v := range messages {
		strmessage[k] = fmt.Sprintf("%v", v)
	}
	message := strings.Join(strmessage, " ")
	strextra := make([]string, len(extra))
	i := 0
	for k, v := range extra {
		strextra[i] = fmt.Sprintf("%s:%v", k, v)
		i++
	}
	_, file, line, _ := runtime.Caller(1)
	extras := map[string]interface{}{
		"file": file,
		"line": line,
		"tags": g.tags,
		"app":  g.appName,
	}
	for k, v := range extra {
		extras[k] = v
	}
	m := &gelf.Message{
		Version:  "1.1",
		Host:     g.hostname,
		Short:    string(message),
		Full:     string(message),
		TimeUnix: float64(time.Now().Unix()),
		Level:    levels[loglevel], // info
		Extra:    extras,
	}

	mBuf := newBuffer()
	defer bufPool.Put(mBuf)

	if err := m.MarshalJSONBuf(mBuf); err != nil {
		log.Println("Error during marshal to JSON")
		return err
	}

	jsonLog := mBuf.Bytes()

	log.Println(string(jsonLog))

	if g.development {
		return nil
	}

	if strings.EqualFold(g.protocol, "tcp") {
		return g.writer.(*gelf.TCPWriter).WriteMessage(m)
	} else {
		gUDPWriter := g.writer.(*gelf.UDPWriter)
		gUDPWriter.CompressionType = 2
		return gUDPWriter.WriteMessage(m)
	}
}

// It will return the correct gelfWriter based on the chosen transport protocol
func GetWriter(protocol, graylogAddr string) (interface{}, error) {

	var err error
	var gelfWriter interface{}

	if strings.EqualFold(protocol, "tcp") {
		gelfWriter, err = gelf.NewTCPWriter(graylogAddr)
		return gelfWriter, err
	}
	if strings.EqualFold(protocol, "udp") {
		gelfWriter, err = gelf.NewUDPWriter(graylogAddr)
		return gelfWriter, err
	}
	errMessage := "Invalid Transport Protocol " + protocol
	return gelfWriter, errors.New(errMessage)
}

// InitLogger Initialize logger with Info Debug and Error
func InitLogger(graylogAddr string, appName string, tags string, development bool, protocol string) {
	var err error
	var gelfWriter interface{}

	if graylogAddr == "" {
		envAddr, ok := os.LookupEnv("GELF_GRAYLOG_SERVER")
		if (!ok && !development) || (envAddr == "" && !development) {
			log.Fatalf("Error! Graylog server not defined.")
			return
		}
		graylogAddr = envAddr
	}
	log.Println("Graylog server: ", graylogAddr)

	gelfWriter, err = GetWriter(protocol, graylogAddr)

	if err != nil {
		log.Fatalf("GelfWriter generation failed: %s", err)
	}

	var host string
	if host, err = os.Hostname(); err != nil {
		host = "undefined"
		log.Println("Hostname undefined!")
	}

	if appName == "" {
		envApp, ok := os.LookupEnv("GELF_APP_NAME")
		if !ok || envApp == "" {
			envApp = "undefined"
			log.Println("Nome de app nao definido. Usando undefined.")
		}
		appName = envApp
	}

	if tags == "" {
		envTags, ok := os.LookupEnv("GELF_TAGS")
		if !ok || envTags == "" {
			envTags = appName
			log.Println("Tags not defined. Using appName.")
		}
		tags = envTags
	}

	Logger = &Gelf{
		writer:      gelfWriter,
		appName:     appName,
		tags:        tags,
		hostname:    host,
		development: development,
		protocol:    protocol,
	}

	if development {
		Logger.SendLog(nil, "INFO", "Logging to stdout")
	} else {
		Logger.SendLog(nil, "INFO", "Logging to stdout")
		Logger.SendLog(nil, "INFO", "Logging to", graylogAddr)
	}
}
