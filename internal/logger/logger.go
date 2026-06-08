package logger

import (
	"net/http"
	"time"

	"goproxy/internal/requestctx"
)

type Entry struct {
	Method     string
	Path       string
	RouteID    string
	StatusCode int
	Duration   time.Duration
	RemoteAddr string
	Timestamp  time.Time
}

type Sink interface {
	Write(Entry)
}

type MultiSink []Sink

func (s MultiSink) Write(entry Entry) {
	for _, sink := range s {
		if sink != nil {
			sink.Write(entry)
		}
	}
}

type ChannelSink struct {
	entries chan Entry
}

func NewChannelSink(size int) *ChannelSink {
	if size < 1 {
		size = 1
	}
	return &ChannelSink{entries: make(chan Entry, size)}
}

func (s *ChannelSink) Write(entry Entry) {
	select {
	case s.entries <- entry:
	default:
	}
}

func (s *ChannelSink) Entries() <-chan Entry {
	return s.entries
}

func Middleware(sink Sink) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fields := &requestctx.LogFields{}
			r = r.WithContext(requestctx.WithLogFields(r.Context(), fields))
			recorder := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			if sink != nil {
				sink.Write(Entry{
					Method:     r.Method,
					Path:       r.URL.Path,
					RouteID:    fields.RouteID,
					StatusCode: recorder.statusCode,
					Duration:   time.Since(start),
					RemoteAddr: r.RemoteAddr,
					Timestamp:  start,
				})
			}
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
