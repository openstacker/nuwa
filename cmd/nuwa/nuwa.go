/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing permissions
and limitations under the License.
*/

package main

import (
	"context"
	"os"
	"path/filepath"
	"net/http"
	"go.uber.org/zap"
	"github.com/julienschmidt/httprouter"
	"github.com/oklog/run"
	"gopkg.in/alecthomas/kingpin.v2"
	"time"

)

func main() {
	var (
		app = kingpin.New(filepath.Base(os.Args[0]), "Automatically cordons and drains nodes that match the supplied conditions.").DefaultEnvars()
		debug            = app.Flag("debug", "Run with debug logging").Short('d').Bool()
		listen           = app.Flag("listen", "Address at which to expose /metrics and /healthz.").Default(":10086").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	log, err := zap.NewProduction()
	if *debug {
		log, err = zap.NewDevelopment()
	}
	kingpin.FatalIfError(err, "cannot create log")
	defer log.Sync()

    log.Debug("Nuwa is listening on %s", zap.String("listen", *listen))
	web := &httpRunner{l: *listen, h: map[string]http.Handler{
		"/healthz": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { r.Body.Close() }), // nolint:gosec
	}}

	log.Debug("test")

	kingpin.FatalIfError(await(web), "error serving")
}

type runner interface {
	Run(stop <-chan struct{})
}

func await(rs ...runner) error {
	stop := make(chan struct{})
	g := &run.Group{}
	for i := range rs {
		r := rs[i] // https://golang.org/doc/faq#closures_and_goroutines
		g.Add(func() error { r.Run(stop); return nil }, func(err error) { close(stop) })
	}
	return g.Run()
}

type httpRunner struct {
	l string
	h map[string]http.Handler
}

func (r *httpRunner) Run(stop <-chan struct{}) {
	rt := httprouter.New()
	for path, handler := range r.h {
		rt.Handler("GET", path, handler)
	}

	s := &http.Server{Addr: r.l, Handler: rt}
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	go func() {
		<-stop
		s.Shutdown(ctx) // nolint:gosec
	}()
	s.ListenAndServe() // nolint:gosec
	cancel()
}