package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/brigadecore/brigade/sdk/v2"
	"github.com/brigadecore/brigade/sdk/v2/core"
	"github.com/brigadecore/brigade/sdk/v2/meta"
	"github.com/brigadecore/brigade/sdk/v2/restmachinery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/willie-yao/brigade-prometheus/v2/exporter/internal/os"
)

var (
	totalRunningJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_running_jobs_total",
		Help: "The total number of processed events",
	})

	totalPendingWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_pending_workers_total",
		Help: "The total number of pending workers",
	})

	totalFailedWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_failed_workers_total",
		Help: "The total number of failed workers",
	})
)

func recordMetrics(client sdk.APIClient) {
	go func() {
		for {
			tempRunningJobs, err :=
				client.Core().Substrate().CountRunningJobs(context.Background())
			if err != nil {
				log.Println(err)
			}
			totalRunningJobs.Set(float64(tempRunningJobs.Count))

			pendingEventsList, err := client.Core().Events().List(
				context.Background(),
				&core.EventsSelector{
					WorkerPhases: []core.WorkerPhase{core.WorkerPhasePending},
				},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			totalPendingWorkers.Set(float64(len(pendingEventsList.Items) +
				int(pendingEventsList.RemainingItemCount)))

			failedEventsList, err := client.Core().Events().List(
				context.Background(),
				&core.EventsSelector{
					WorkerPhases: []core.WorkerPhase{core.WorkerPhaseFailed},
				},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			totalFailedWorkers.Set(float64(len(failedEventsList.Items) +
				int(failedEventsList.RemainingItemCount)))

			time.Sleep(2 * time.Second)
		}
	}()
}

func main() {

	// The address of the Brigade 2 API server
	// beginning with http:// or https//
	apiAddress, err := os.GetRequiredEnvVar("API_ADDRESS")
	if err != nil {
		log.Fatal(err)
	}

	// An API token obtained using the Brigade 2 CLI
	apiToken, err := os.GetRequiredEnvVar("API_TOKEN")
	if err != nil {
		log.Fatal(err)
	}

	// Boolean indicating whether or not to ignore SSL errors
	apiIgnoreCertWarnings, err := os.GetBoolFromEnvVar("API_IGNORE_CERT_WARNINGS", true)
	if err != nil {
		log.Println(err)
	}

	// Instantiate the API Client
	client := sdk.NewAPIClient(
		apiAddress,
		apiToken,
		&restmachinery.APIClientOptions{
			AllowInsecureConnections: apiIgnoreCertWarnings,
		},
	)

	recordMetrics(client)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
