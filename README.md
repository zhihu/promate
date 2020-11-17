# Promate - Graphite On VictoriaMetrics

Promate is a high-performance graphite storage solution.

Compare with Whisper:

* 10x faster on average; 60-100x faster for complex, long range queries
* 90% storage space reduction, 99.99% IOPS reduction
* 80% reduction in memory and CPU overhead with constant query pressure

This is a comparison of performance from our production environment. Welcome to help us design tests that give reproducible benchmark results.

### Features

* Higher performance with lower cpu, memory, and storage usage, benefit from the excellent [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)
* Supports almost all graphite functions, benefit from compatible with [carbonapi](https://github.com/go-graphite/carbonapi)
* MateQL language, support query graphite metrics with PromQL
* Real-time aggregation, no loss of accuracy of historical metrics

### Architecture
![Overview](docs/arch.png)

### Example Config

1. [carbonapi.yaml](examples/carbonapi.yaml)
1. [matecarbon.yaml](examples/matecarbon.yaml)

### Thanks

* [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) & [metricsql](https://github.com/VictoriaMetrics/metricsql)
* [carbonapi](https://github.com/go-graphite/carbonapi)
* [m3](https://github.com/m3db/m3)

### License

[Apache License 2.0](LICENSE.txt)
