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

	totalStartingWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_starting_workers_total",
		Help: "The total number of starting workers",
	})

	totalRunningWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_running_workers_total",
		Help: "The total number of running workers",
	})

	totalPendingWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_pending_workers_total",
		Help: "The total number of pending workers",
	})

	totalFailedWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_failed_workers_total",
		Help: "The total number of failed workers",
	})

	totalAbortedWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_aborted_workers_total",
		Help: "The total number of aborted workers",
	})

	totalCanceledWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_canceled_workers_total",
		Help: "The total number of canceled workers",
	})

	totalSucceededWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_succeeded_workers_total",
		Help: "The total number of succeeded workers",
	})

	totalTimedOutWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_timed_out_workers_total",
		Help: "The total number of timed-out workers",
	})

	totalSchedulingFailedWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_scheduling_failed_workers_total",
		Help: "The total number of scheduling-failed workers",
	})

	totalUnknownWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_unknown_workers_total",
		Help: "The total number of unknown workers",
	})

	runningWorkersByProject = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_running_workers_by_project",
		Help: "The number of running workers by project",
	}, []string{"projectID"})
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

			recordWorkerGaugeMetric(client, totalStartingWorkers, core.WorkerPhaseStarting)
			recordWorkerGaugeMetric(client, totalRunningWorkers, core.WorkerPhaseRunning)
			recordWorkerGaugeMetric(client, totalPendingWorkers, core.WorkerPhasePending)
			recordWorkerGaugeMetric(client, totalFailedWorkers, core.WorkerPhaseFailed)
			recordWorkerGaugeMetric(client, totalAbortedWorkers, core.WorkerPhaseAborted)
			recordWorkerGaugeMetric(client, totalCanceledWorkers, core.WorkerPhaseCanceled)
			recordWorkerGaugeMetric(client, totalSucceededWorkers, core.WorkerPhaseSucceeded)
			recordWorkerGaugeMetric(client, totalTimedOutWorkers, core.WorkerPhaseTimedOut)
			recordWorkerGaugeMetric(client, totalSchedulingFailedWorkers, core.WorkerPhaseSchedulingFailed)
			recordWorkerGaugeMetric(client, totalUnknownWorkers, core.WorkerPhaseUnknown)

			runningList, err := client.Core().Events().List(
				context.Background(),
				&core.EventsSelector{
					WorkerPhases: []core.WorkerPhase{core.WorkerPhaseRunning},
				},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			eventMapByProjectID := make(map[string][]core.Event)

			for _, event := range runningList.Items {
				eventMapByProjectID[event.ProjectID] =
					append(eventMapByProjectID[event.ProjectID], event)
			}

			for projectID, workerList := range eventMapByProjectID {
				runningWorkersByProject.With(
					prometheus.Labels{"projectID": projectID},
				).Set(float64(len(workerList)))
			}

			time.Sleep(2 * time.Second)
		}
	}()
}

func recordWorkerGaugeMetric(client sdk.APIClient, gauge prometheus.Gauge, phase core.WorkerPhase) {
	eventList, err := client.Core().Events().List(
		context.Background(),
		&core.EventsSelector{
			WorkerPhases: []core.WorkerPhase{phase},
		},
		&meta.ListOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}

	gauge.Set(float64(len(eventList.Items) +
		int(eventList.RemainingItemCount)))
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
	apiIgnoreCertWarnings, err :=
		os.GetBoolFromEnvVar("API_IGNORE_CERT_WARNINGS", true)
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
