package main

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace = "dns_exporter"
)

type Config struct {
	Hosts []string
}

type DNSCollector struct {
	total      *prometheus.Desc
	totalError *prometheus.Desc
	latency    *prometheus.Desc

	hosts []string

	totalCount      map[string]int
	totalCountMutex sync.Mutex

	totalErrorCount      map[string]int
	totalErrorCountMutex sync.Mutex
}

func NewDNSCollector(config Config) (*DNSCollector, error) {
	dnsCollector := &DNSCollector{
		total: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "resolution_total"),
			"Total number of DNS resolutions.",
			[]string{"host"},
			nil,
		),
		totalError: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "resolution_error_total"),
			"Total number of DNS resolution errors.",
			[]string{"host"},
			nil,
		),
		latency: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "resolution_seconds"),
			"Time taken to resolve DNS.",
			[]string{"host"},
			nil,
		),

		hosts: config.Hosts,

		totalCount:      map[string]int{},
		totalErrorCount: map[string]int{},
	}

	return dnsCollector, nil
}

func (e *DNSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.total
	ch <- e.totalError
	ch <- e.latency
}

func (e *DNSCollector) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup

	wg.Add(len(e.hosts))

	for _, host := range e.hosts {
		go func(host string) {
			defer wg.Done()

			e.resolveHost(ch, host)
		}(host)
	}

	wg.Wait()
}

func (e *DNSCollector) resolveHost(ch chan<- prometheus.Metric, host string) {
	start := time.Now()

	_, err := net.LookupHost(host)
	if err != nil {
		e.totalErrorCountMutex.Lock()
		e.totalErrorCount[host] += 1
		e.totalErrorCountMutex.Unlock()

		log.Printf("could not lookup host '%s': %s", host, err)
	}

	elapsed := time.Since(start)

	e.totalCountMutex.Lock()
	e.totalCount[host] += 1
	e.totalCountMutex.Unlock()

	ch <- prometheus.MustNewConstMetric(e.total, prometheus.CounterValue, float64(e.totalCount[host]), host)
	ch <- prometheus.MustNewConstMetric(e.totalError, prometheus.CounterValue, float64(e.totalErrorCount[host]), host)
	ch <- prometheus.MustNewConstMetric(e.latency, prometheus.GaugeValue, elapsed.Seconds(), host)
}

func main() {
	dnsCollector, err := NewDNSCollector(Config{
		Hosts: []string{
			"example.org",
			"google.com",
		},
	})
	if err != nil {
		log.Fatalf("could not create dns collector: %s", err)
	}

	prometheus.MustRegister(dnsCollector)

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe("localhost:8000", nil)
}
