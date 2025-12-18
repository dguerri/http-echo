// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/http-echo/version"
)

const (
	httpHeaderAppName    string = "X-App-Name"
	httpHeaderAppVersion string = "X-App-Version"

	httpLogDateFormat        string = "2006/01/02 15:04:05"
	httpLogFormat            string = "%v %s %s \"%s %s %s\" %d %d \"%s\" %v\n"
	httpLogFormatWithForward string = "%v %s %s %s \"%s %s %s\" %d %d \"%s\" %v\n"
	httpLogEnvForwardedFor   string = "LOG_FORWARDED_FOR"
)

// withAppHeaders adds application headers such as X-App-Version and X-App-Name.
func withAppHeaders(c int, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(httpHeaderAppName, version.Name)
		w.Header().Set(httpHeaderAppVersion, version.Version)
		w.WriteHeader(c)
		h(w, r)
	}
}

// metaResponseWriter is a response writer that saves information about the
// response for logging.
type metaResponseWriter struct {
	writer http.ResponseWriter
	status int
	length int
}

// Header implements the http.ResponseWriter interface.
func (w *metaResponseWriter) Header() http.Header {
	return w.writer.Header()
}

// WriteHeader implements the http.ResponseWriter interface.
func (w *metaResponseWriter) WriteHeader(s int) {
	w.status = s
	w.writer.WriteHeader(s)
}

// Write implements the http.ResponseWriter interface.
func (w *metaResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.length = len(b)
	return w.writer.Write(b)
}

// httpLog accepts an io object and logs the request and response objects to the
// given io.Writer.
func httpLog(out io.Writer, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var mrw metaResponseWriter
		mrw.writer = w

		defer func(start time.Time) {
			status := mrw.status
			length := mrw.length
			end := time.Now()
			dur := end.Sub(start)

			if os.Getenv(httpLogEnvForwardedFor) != "" {
				forwardedFor := r.Header.Get("X-Forwarded-For")
				if forwardedFor != "" {
					// Extract first IP from comma-separated list
					if idx := strings.Index(forwardedFor, ","); idx != -1 {
						forwardedFor = strings.TrimSpace(forwardedFor[:idx])
					}
				} else {
					forwardedFor = "-"
				}
				fmt.Fprintf(out, httpLogFormatWithForward,
					end.Format(httpLogDateFormat),
					r.Host, r.RemoteAddr, forwardedFor, r.Method, r.URL.Path, r.Proto,
					status, length, r.UserAgent(), dur)
			} else {
				fmt.Fprintf(out, httpLogFormat,
					end.Format(httpLogDateFormat),
					r.Host, r.RemoteAddr, r.Method, r.URL.Path, r.Proto,
					status, length, r.UserAgent(), dur)
			}
		}(time.Now())

		h(&mrw, r)
	}
}
