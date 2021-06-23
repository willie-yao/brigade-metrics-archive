package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/brigadecore/brigade/sdk/v2"
	"github.com/brigadecore/brigade/sdk/v2/authn"
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

	allWorkersByPhase = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_workers_by_phase",
		Help: "All workers separated by phase",
	}, []string{"workerPhase"})

	allJobsByPhase = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_jobs_by_phase",
		Help: "All jobs separated by phase",
	}, []string{"jobPhase"})

	allRunningJobsDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_running_jobs_duration",
		Help: "The duration of all running jobs",
	}, []string{"job"})

	allRunningWorkersDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_running_workers_duration",
		Help: "The duration of all running workers",
	}, []string{"worker"})

	totalUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_users_total",
		Help: "The total number of users",
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

			// brigade_all_workers_by_phase
			eventsList, err := client.Core().Events().List(
				context.Background(),
				&core.EventsSelector{},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			eventMapByStatus := make(map[core.WorkerPhase][]core.Event)

			for _, event := range eventsList.Items {
				eventMapByStatus[event.Worker.Status.Phase] =
					append(eventMapByStatus[event.Worker.Status.Phase], event)
			}

			for workerPhase, workerList := range eventMapByStatus {
				allWorkersByPhase.With(
					prometheus.Labels{"workerPhase": string(workerPhase)},
				).Set(float64(len(workerList)))
			}

			// brigade_all_running_workers_duration
			for _, worker := range eventMapByStatus[core.WorkerPhaseRunning] {
				allRunningWorkersDuration.With(
					prometheus.Labels{"worker": worker.ID},
				).Set(time.Since(*worker.Worker.Status.Started).Seconds())
			}

			// brigade_all_running_jobs_duration
			var jobsMapByStatus = make(map[core.JobPhase][]core.Job)
			var runningJobsList []core.Job

			for _, event := range eventsList.Items {
				if event.Worker.Status.Phase == core.WorkerPhaseRunning {
					runningJobsList = append(runningJobsList, event.Worker.Jobs...)
				}
				for _, job := range event.Worker.Jobs {
					jobsMapByStatus[job.Status.Phase] =
						append(jobsMapByStatus[job.Status.Phase], job)
				}
			}

			for _, job := range runningJobsList {
				allRunningJobsDuration.With(
					prometheus.Labels{"job": job.Name},
				).Set(time.Since(*job.Status.Started).Seconds())
			}

			// brigade_all_running_jobs_duration
			for jobPhase, jobList := range jobsMapByStatus {
				allJobsByPhase.With(
					prometheus.Labels{"jobPhase": string(jobPhase)},
				).Set(float64(len(jobList)))
			}

			// brigade_users_total
			userList, err := client.Authn().Users().List(
				context.Background(),
				&authn.UsersSelector{},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			totalUsers.Set(float64(len(userList.Items) +
				int(userList.RemainingItemCount)))

			time.Sleep(5 * time.Second)
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
