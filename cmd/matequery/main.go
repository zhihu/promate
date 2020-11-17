package main

import (
	"flag"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/zhihu/promate/prometheus"
)

func main() {
	var logLevel, listenAddr, prometheusURL string
	flag.StringVar(&logLevel, "logLevel", "info", "log level")
	flag.StringVar(&listenAddr, "listenAddr", ":8481", "listen address")
	flag.StringVar(&prometheusURL, "prometheusURL", "", "prometheus query address")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)

	target, err := url.Parse(prometheusURL)
	if err != nil {
		log.Fatal(err)
	}

	router := chi.NewRouter()

	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.RealIP)

	router.Get("/check_health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("zhi~"))
	})

	router.Mount("/debug", middleware.Profiler())

	router.Handle("/*", &httputil.ReverseProxy{Director: func(req *http.Request) {
		req.Host = target.Host
		req.URL.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		reqQuery := req.URL.Query()
		for _, key := range []string{"query", "match[]"} {
			if queries, ok := reqQuery[key]; ok {
				reqQuery.Del(key)
				for _, query := range queries {
					mateQuery, err := prometheus.CovertMateQuery(query, key == "query")
					if err != nil {
						reqQuery.Add(key, query)
						log.Errorf("covert %s failed %s", query, err)
					} else {
						reqQuery.Add(key, mateQuery)
						log.Infof("covert %s -> %s", query, mateQuery)
					}
				}
			}
		}
		req.URL.RawQuery = reqQuery.Encode()
	}})

	log.Fatal(http.ListenAndServe(listenAddr, router))
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
