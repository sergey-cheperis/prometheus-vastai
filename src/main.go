package main

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag(
		"listen",
		"Address to listen on.",
	).Default("0.0.0.0:8622").String()
	apiKey = kingpin.Flag(
		"key",
		"Vast.ai API key",
	).Default("").String()
	updateInterval = kingpin.Flag(
		"update-interval",
		"How often to query Vast.ai for updates",
	).Default("1m").Duration()
	stateDir = kingpin.Flag(
		"state-dir",
		"Path to store state files (default $HOME)",
	).String()
	masterUrl = kingpin.Flag(
		"master-url",
		"Query global data from the master exporter and not from Vast.ai directly.",
	).String()
)

func metricsHandler(w http.ResponseWriter, r *http.Request, collector prometheus.Collector) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	kingpin.Version(version.Print("vastai_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting vast.ai exporter")

	if *stateDir == "" {
		*stateDir = os.Getenv("HOME")
	}
	if *stateDir == "" {
		*stateDir = "/tmp"
	}

	log.Infoln("Reading initial Vast.ai info")

	// read info from vast.ai: offers
	info := getVastAiInfo(*masterUrl)
	err := offerCache.InitialUpdateFrom(info)
	if err != nil {
		// initial update must succeed, otherwise exit
		log.Fatalln(err)
	}

	// read info from vast.ai: global stats
	vastAiGlobalCollector := newVastAiGlobalCollector()
	vastAiGlobalCollector.UpdateFrom(&offerCache)

	// read info from vast.ai: account stats (if api key is specified)
	useAccount := *apiKey != ""
	vastAiAccountCollector := newVastAiAccountCollector()
	if useAccount {
		err = vastAiAccountCollector.InitialUpdateFrom(info, &offerCache)
		if err != nil {
			// initial update must succeed, otherwise exit
			log.Fatalln(err)
		}
	} else {
		log.Infoln("No Vast.ai API key provided, only serving global stats")
	}

	http.HandleFunc("/offers", func(w http.ResponseWriter, r *http.Request) {
		// json list of offers
		w.Header().Set("Content-Type", "application/json")
		w.Write(offerCache.rawOffersJson(false))
	})
	http.HandleFunc("/machines", func(w http.ResponseWriter, r *http.Request) {
		// json list of machines
		w.Header().Set("Content-Type", "application/json")
		w.Write(offerCache.rawOffersJson(true))
	})
	http.HandleFunc("/metrics/global", func(w http.ResponseWriter, r *http.Request) {
		// global stats
		metricsHandler(w, r, vastAiGlobalCollector)
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// account stats (if api key is specified)
		if useAccount {
			metricsHandler(w, r, vastAiAccountCollector)
		} else {
			metricsHandler(w, r, vastAiGlobalCollector)
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// index page
		w.Write([]byte(`<html><head><title>Vast.ai Exporter</title></head><body><h1>Vast.ai Exporter</h1>`))
		if useAccount {
			w.Write([]byte(`<a href="/metrics">Account stats</a><br><a href="/metrics/global">Global stats</a><br>`))
		} else {
			w.Write([]byte(`<a href="/metrics">Global stats</a><br>`))
		}
		w.Write([]byte(`<a href="/offers">Global JSON list of offers</a><br><a href="/machines">Global JSON list of machines</a><br></body></html>`))
	})

	go func() {
		for {
			time.Sleep(*updateInterval)
			info := getVastAiInfo(*masterUrl)
			offerCache.UpdateFrom(info)
			vastAiGlobalCollector.UpdateFrom(&offerCache)
			if useAccount {
				vastAiAccountCollector.UpdateFrom(info, &offerCache)
			}
		}
	}()

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
