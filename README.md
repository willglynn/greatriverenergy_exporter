# `greatriverenergy_exporter`

This is a Prometheus/OpenMetrics exporter for [Great River Energy](https://greatriverenergy.com), specifically the
[load management data](https://lmguide.grenergy.com).

## Quickstart

Container images are available at [Docker Hub](https://hub.docker.com/r/willglynn/greatriverenergy_exporter) and [GitHub
container registry](https://github.com/willglynn/purpleair_exporter/pkgs/container/greatriverenergy_exporter).

```shell
$ docker run -it --rm -p 2024:2024 willglynn/greatriverenergy_exporter
# or
$ docker run -it --rm -p 2024:2024 ghcr.io/willglynn/greatriverenergy_exporter
2023/07/08 11:07:08 Starting HTTP server on :2024
```

## Endpoints

The metrics endpoint at [`GET /metrics`]([http://localhost:2024/metrics]) returns the
[schedule](https://lmguide.grenergy.com) and [shed counts](https://lmguide.grenergy.com/ShedCount.aspx):

```text
# HELP greatriverenergy_conservation_gauge An indicator of electric transmission system load versus capacity. 1 = Normal, 2 = Elevated, 3 = Peak, 4 = Critical
# TYPE greatriverenergy_conservation_gauge gauge
greatriverenergy_conservation_gauge 1
# HELP greatriverenergy_shed_count The number of times a load shedding event occurred
# TYPE greatriverenergy_shed_count counter
greatriverenergy_shed_count{program="C&I Interruptible Metered"} 35
greatriverenergy_shed_count{program="C&I with GenSet"} 35
greatriverenergy_shed_count{program="Critical peak pricing"} 0
greatriverenergy_shed_count{program="Cycled Air Conditioning"} 152
greatriverenergy_shed_count{program="Dual Fuel"} 214
greatriverenergy_shed_count{program="Dual Fuel Fall Test"} 13
greatriverenergy_shed_count{program="Dual Fuel Nick Test"} 1
greatriverenergy_shed_count{program="Interruptible Crop Driers"} 20
greatriverenergy_shed_count{program="Interruptible Irrigation"} 160
greatriverenergy_shed_count{program="Interruptible Water Heating"} 300
greatriverenergy_shed_count{program="Lake Country Power Dual Fuel"} 14
greatriverenergy_shed_count{program="Lake Country Power Interruptible Water"} 18
greatriverenergy_shed_count{program="Public Appeal for Conservation"} 1
# HELP greatriverenergy_shed_count_reset_on The date at which the shed counts were last reset
# TYPE greatriverenergy_shed_count_reset_on gauge
greatriverenergy_shed_count_reset_on 1.3896792e+09
# HELP greatriverenergy_shed_likelihood An indicator of the likelihood of using a load shedding program. 1 = Unlikely, 2 = Possible, 3 = Likely, 4 = Scheduled
# TYPE greatriverenergy_shed_likelihood gauge
greatriverenergy_shed_likelihood{program="C&I Interruptible Metered",when="next_day"} 1
greatriverenergy_shed_likelihood{program="C&I Interruptible Metered",when="today"} 1
greatriverenergy_shed_likelihood{program="C&I with GenSet",when="next_day"} 1
greatriverenergy_shed_likelihood{program="C&I with GenSet",when="today"} 1
greatriverenergy_shed_likelihood{program="Cycled Air Conditioning",when="next_day"} 1
greatriverenergy_shed_likelihood{program="Cycled Air Conditioning",when="today"} 1
greatriverenergy_shed_likelihood{program="Interruptible Irrigation",when="next_day"} 1
greatriverenergy_shed_likelihood{program="Interruptible Irrigation",when="today"} 1
greatriverenergy_shed_likelihood{program="Interruptible Water Heating",when="next_day"} 1
greatriverenergy_shed_likelihood{program="Interruptible Water Heating",when="today"} 1
```

The history endpoint at [`GET /history?days=7`](http://localhost:2024/history?days=7) returns actual load management
events, providing values 0 the minute before, 1 every minute during the event, and 0 the minute after the event has
finished:

```text
# HELP greatriverenergy_shed_event A load shedding event that occurred
# TYPE greatriverenergy_shed_event gauge
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 0 1688417940000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688418000000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688418060000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688418120000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688418180000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688418240000
…
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688432280000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 1 1688432340000
greatriverenergy_shed_event{class="CI",program="Interruptible Irrigation"} 0 1688432460000
greatriverenergy_shed_event{class="R",program="Cycled Air Conditioning"} 0 1688414340000
greatriverenergy_shed_event{class="R",program="Cycled Air Conditioning"} 1 1688414400000
greatriverenergy_shed_event{class="R",program="Cycled Air Conditioning"} 1 1688414460000
…
```

If you have a [sufficiently flexible data store](https://docs.victoriametrics.com/#backfilling), you can use this
endpoint to backfill a decade of historical events all at once.

```console
% curl http://localhost:2024/history\?days=3650 -o shed_events.txt
% wc -l shed_events.txt 
  260267 shed_events.txt
% curl -X POST http://localhost:8428/api/v1/import/prometheus -T shed_events.txt 
```

## Prometheus configuration

A good starting point:

```yaml
scrape_configs:
  - job_name: 'greatriverenergy'
    scrape_interval: 2m
    static_configs:
      - targets:
          - http://localhost:2024
    metric_relabel_configs:
      - if: '{__name__=~"^greatriverenergy_.*"}'
        action: labeldrop
        regex: "instance|job"

  - job_name: 'greatriverenergy_history'
    scrape_interval: 1h
    metrics_path: /history
    static_configs:
      - targets:
          - http://localhost:2024
    metric_relabel_configs:
      - if: '{__name__=~"^greatriverenergy_.*"}'
        action: labeldrop
        regex: "instance|job"
```
