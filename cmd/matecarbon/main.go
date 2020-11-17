package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	protov3 "github.com/go-graphite/protocol/carbonapi_v3_pb"
	"github.com/imroc/req"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"github.com/zhihu/promate/prometheus"
	"gopkg.in/yaml.v3"
)

var defaultMaxDatapoints float64 = 1024
var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RollupConfig struct {
	MatchSuffix   string         `yaml:"match_suffix"`
	MatchSuffixRe *regexp.Regexp `yaml:"-"`
	RollupFunc    string         `yaml:"rollup_func"`
}

type Config struct {
	Listen              string          `yaml:"listen"`
	LogLevel            log.Level       `yaml:"-"`
	StatsdFlushInterval float64         `yaml:"statsd_flush_interval"`
	PrometheusURL       string          `yaml:"prometheus_url"`
	PrometheusMaxBody   int64           `yaml:"prometheus_max_body"`
	Rollups             []*RollupConfig `yaml:"rollups"`
	DefaultRollupFunc   string          `yaml:"default_rollup_func"`
}

func LoadConfig(configPath string) (*Config, error) {
	body, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	config := new(Config)
	err = yaml.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}
	for _, rollup := range config.Rollups {
		rollup.MatchSuffixRe, err = regexp.Compile(fmt.Sprintf("%s$", rollup.MatchSuffix))
		if err != nil {
			return nil, err
		}
	}
	return config, err
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "matecarbon.yaml", "config file path")
	flag.Parse()

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	wrapper := newWrapper(config)

	router := chi.NewRouter()

	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.RealIP)

	router.Get("/check_health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("zhi~"))
	})

	router.Mount("/debug", middleware.Profiler())

	router.Get("/metrics/find/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		multiRequest := &protov3.MultiGlobRequest{}
		err = multiRequest.Unmarshal(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		multiResponse, err := wrapper.Find(r.Context(), multiRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		blob, err := multiResponse.Marshal()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(blob)
	})

	router.Get("/render/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		multiRequest := &protov3.MultiFetchRequest{}
		err = multiRequest.Unmarshal(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		multiResponse, err := wrapper.Render(r.Context(), multiRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		blob, err := multiResponse.Marshal()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(blob)
	})

	log.Fatal(http.ListenAndServe(config.Listen, router))
}

func newWrapper(config *Config) *Wrapper {
	request := req.New()
	request.SetClient(&http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   1 * time.Second,
				KeepAlive: 1 * time.Second,
			}).DialContext,
			MaxIdleConns:          0,
			MaxIdleConnsPerHost:   0,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   1 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		// Don't worry about the request taking too long.
		// It will end when the user's request context cancel or VictoriaMetrics timeout.
		Timeout: time.Minute * 10,
	})
	return &Wrapper{
		config:  config,
		request: request,
	}
}

type Wrapper struct {
	config  *Config
	request *req.Req
}

func (w *Wrapper) Find(ctx context.Context, multiRequest *protov3.MultiGlobRequest) (multiResponse *protov3.MultiGlobResponse, err error) {
	var wg sync.WaitGroup
	var lock sync.Mutex

	multiResponse = &protov3.MultiGlobResponse{
		Metrics: make([]protov3.GlobResponse, 0),
	}

	for _, target := range multiRequest.Metrics {
		wg.Add(1)
		go func(target string) {
			logger := log.WithFields(log.Fields{
				"type": "find",
				"path": target,
			})

			startTime := time.Now()
			defer func() {
				wg.Done()

				logger.Infof("took %s", time.Since(startTime))
			}()

			// We can't query the full amount of metrics, which can cause serious performance issues.
			// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/329#issuecomment-590773944
			if target == "*" {
				logger.Warnf("can't query full amount metrics")
				return
			}

			// Too large a request will cause the prometheus backend to fail to respond.
			// This is usually caused by the automatic expansion of the All option of the Grafana variable.
			// Most of the time you can customize the value of the All option to be * .
			// Now the carbonapi_v3_pb protocol doesn't return custom errors, so it's ignored here.
			if len(target) > 8192 {
				logger.Errorf("path too long")
				return
			}

			var params req.Param

			name, filters := prometheus.ConvertGraphiteTarget(target, false)
			selector := filters.Build(name)

			prefix, query, fast := prometheus.ConvertQueryLabel(target)
			// In VictoriaMetrics, query is faster without query params.
			// We can use this approach for the second segment of our graphite metrics.
			// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/359#issuecomment-596098714
			if !fast {
				params = req.Param{
					"start":   multiRequest.StartTime,
					"end":     multiRequest.StopTime,
					"match[]": selector,
				}
			}

			resp, err := w.request.Get(fmt.Sprintf("%s/api/v1/label/%s/values", w.config.PrometheusURL, query), ctx, params)
			if err != nil {
				logger.Errorf("request failed %s", err)
				return
			}

			defer func() { _ = resp.Response().Body.Close() }()

			// We restrict particularly large responses to queries that can use MateQL.
			body, err := ioutil.ReadAll(io.LimitReader(resp.Response().Body, w.config.PrometheusMaxBody))
			if err != nil {
				logger.Errorf("read response failed %s", err)
				return
			}

			data := new(prometheus.ValuesResponse)
			err = json.Unmarshal(body, data)
			if err != nil {
				logger.Errorf("unmarshal %s failed %s", string(body), err)
				return
			}

			metric := protov3.GlobResponse{
				Name:    target,
				Matches: make([]protov3.GlobMatch, 0, len(data.Data)),
			}
			for _, label := range data.Data {
				metric.Matches = append(metric.Matches, protov3.GlobMatch{
					IsLeaf: false,
					Path:   prefix + label,
				})
			}

			lock.Lock()
			multiResponse.Metrics = append(multiResponse.Metrics, metric)
			lock.Unlock()
		}(target)
	}

	wg.Wait()
	return multiResponse, nil
}

func (w *Wrapper) Render(ctx context.Context, multiRequest *protov3.MultiFetchRequest) (multiResponse *protov3.MultiFetchResponse, err error) {
	var wg sync.WaitGroup
	var locker sync.Mutex

	multiResponse = &protov3.MultiFetchResponse{
		Metrics: make([]protov3.FetchResponse, 0),
	}
	for _, request := range multiRequest.Metrics {
		wg.Add(1)
		go func(request protov3.FetchRequest) {
			logger := log.WithFields(log.Fields{
				"type":            "render",
				"start":           request.StartTime,
				"end":             request.StopTime,
				"path":            request.PathExpression,
				"max_data_points": request.MaxDataPoints,
			})

			startTime := time.Now()
			defer func() {
				wg.Done()

				logger.Infof("took %s", time.Since(startTime))
			}()

			// For the same reasons as above.
			if len(request.PathExpression) > 8192 {
				logger.Errorf("path too long")
				return
			}

			name, filters := prometheus.ConvertGraphiteTarget(request.PathExpression, true)
			selector := filters.Build(name)

			// The default value is used when the request does not take the MaxDataPoints.
			// Most of these requests come from scripts, not Grafana.
			maxDataPoints := float64(request.MaxDataPoints)
			if maxDataPoints == 0 {
				maxDataPoints = defaultMaxDatapoints
			}
			timeRange := float64(request.StopTime - request.StartTime)
			// Try to set step to a multiple of the statsd flush interval.
			// Otherwise the returned result will be jittery.
			multipleInterval := math.Ceil(timeRange/maxDataPoints/w.config.StatsdFlushInterval) * w.config.StatsdFlushInterval
			step := math.Max(multipleInterval, w.config.StatsdFlushInterval)
			window := fmt.Sprintf("%ds", int(step))

			// Similar to carbon's storage aggregation strategy, but in real time. https://graphite.readthedocs.io/en/latest/config-carbon.html#storage-aggregation-conf
			// Here the aggregation strategy is chosen based on queries rather than stored metrics.
			// So there is no way to configure it based on the full metrics name, only the suffix.
			// In future the aggregation strategy needs to be determined based on the incoming consolidateBy function, not just the pre-configuration file.
			// Which would require the carbonapi_v3_pb protocol to pass the aggregation strategy.
			var query string
			for _, rollup := range w.config.Rollups {
				if rollup.MatchSuffixRe.MatchString(request.PathExpression) {
					query = fmt.Sprintf(`%s(%s[%ds])`, rollup.RollupFunc, selector, int(step))
					break
				}
			}
			if query == "" {
				query = fmt.Sprintf(`%s(%s[%ds])`, w.config.DefaultRollupFunc, selector, int(step))
			}

			// In graphite, we do the downscaling in step window size
			// This is different in VictoriaMetrics, so we need to specify the window size for the calculation with max_lookback.
			// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/549#issuecomment-653643283
			params := req.Param{
				"query":        query,
				"start":        request.StartTime,
				"end":          request.StopTime,
				"step":         window,
				"max_lookback": window,
			}

			resp, err := w.request.Get(fmt.Sprintf("%s/api/v1/query_range", w.config.PrometheusURL), ctx, params)
			if err != nil {
				logger.Errorf("request failed %s", err)
				return
			}

			defer func() { _ = resp.Response().Body.Close() }()

			// We restrict particularly large responses to queries that can use MateQL.
			body, err := ioutil.ReadAll(io.LimitReader(resp.Response().Body, w.config.PrometheusMaxBody))
			if err != nil {
				logger.Errorf("read response failed %s", err)
				return
			}

			data := new(prometheus.MatrixResponse)
			err = json.Unmarshal(body, data)
			if err != nil {
				logger.Errorf("unmarshal %s failed %s", string(body), err)
				return
			}

			for _, m := range data.Data.Result {
				// Sometimes the VictoriaMetrics adjustment logic return empty values that we can just ignore.
				if len(m.Values) == 0 {
					continue
				}

				target := prometheus.ConvertPrometheusMetric(name, m.Metric)
				if target == "" {
					logger.Errorf("convert name:%s metric:%s to target failed", name, m.Metric)
					continue
				}

				start := m.Values[0].Timestamp
				end := m.Values[len(m.Values)-1].Timestamp
				count := (end-start)/step + 1

				// The Prometheus response data is not continuous, we populate all intervals with Nan values.
				values := make([]float64, int(count))
				var i, j int
				for ; i < len(values); i++ {
					values[i] = math.NaN()
				searchValue:
					for ; j < len(m.Values); j++ {
						if start+float64(i)*step != m.Values[j].Timestamp {
							break searchValue
						}
						values[i] = m.Values[j].Value
					}
				}

				// Align the start and end points of the metric with the time of the request.
				// Otherwise the division calculation in carbonapi will fail.
				metricStart, metricEnd, metricStep := int64(start), int64(end), int64(step)
				requestStart, requestEnd := request.StartTime, request.StopTime
				if metricStart < requestStart {
					startStep := int64(math.Ceil(float64(requestStart-metricStart) / float64(metricStep)))
					metricStart = metricStart + startStep*metricStep
					values = values[startStep:]
				} else {
					startStep := (metricStart - requestStart) / metricStep
					metricStart = metricStart - startStep*metricStep
					values = append(makeNanArr(startStep), values...)
				}
				if metricEnd > requestEnd {
					stopStep := int64(math.Ceil(float64(metricEnd-requestEnd) / float64(metricStep)))
					metricEnd = metricEnd - stopStep*metricStep
					values = values[:int64(len(values))-stopStep]
				} else {
					stopStep := (requestEnd-requestStart)/metricStep + 1 - int64(len(values))
					metricEnd = metricEnd + stopStep*metricStep
					values = append(values, makeNanArr(stopStep)...)
				}

				// ConsolidationFunc is the consolidation strategy chosen by carbonapi to avoid exceeding MaxDataPoints in response to data.
				// It can be modified by the function consolidateBy. https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.consolidateBy
				// But now the query step is dynamic and response points must not exceed MaxDataPoints, so this configuration or function becomes unnecessary.
				consolidationFunc := "avg"

				metric := protov3.FetchResponse{
					Name:              target,
					PathExpression:    request.PathExpression,
					RequestStartTime:  requestStart,
					RequestStopTime:   requestEnd,
					ConsolidationFunc: consolidationFunc,
					StartTime:         metricStart,
					StopTime:          metricEnd,
					StepTime:          metricStep,
					Values:            values,
				}

				locker.Lock()
				multiResponse.Metrics = append(multiResponse.Metrics, metric)
				locker.Unlock()
			}
		}(request)
	}

	wg.Wait()
	return multiResponse, nil
}

func makeNanArr(count int64) []float64 {
	arr := make([]float64, count)
	for i := int64(0); i < count; i++ {
		arr[i] = math.NaN()
	}
	return arr
}
