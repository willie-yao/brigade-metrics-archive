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
	"github.com/willie-yao/brigade-metrics/exporter/internal/os"
)

var (
	totalRunningJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_running_jobs_total",
		Help: "The total number of running jobs",
	})

	totalPendingJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_pending_jobs_total",
		Help: "The total number of pending jobs",
	})

	allWorkersByPhase = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_workers_by_phase",
		Help: "All workers separated by phase",
	}, []string{"workerPhase"})

	allRunningWorkersDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "brigade_all_running_workers_duration",
		Help: "The duration of all running workers",
	}, []string{"worker"})

	totalUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_users_total",
		Help: "The total number of users",
	})

	totalServiceAccounts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_service_accounts_total",
		Help: "The total number of service accounts",
	})

	totalProjects = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_projects_total",
		Help: "The total number of brigade projects",
	})
)

func recordMetrics(client sdk.APIClient, scrapeInterval int) {
	go func() {
		for {
			// brigade_running_jobs_total
			tempRunningJobs, err :=
				client.Core().Substrate().CountRunningJobs(context.Background())
			if err != nil {
				log.Println(err)
			}
			totalRunningJobs.Set(float64(tempRunningJobs.Count))

			for _, phase := range core.WorkerPhasesAll() {
				// brigade_all_workers_by_phase
				eventsList, err := client.Core().Events().List(
					context.Background(),
					&core.EventsSelector{
						WorkerPhases: []core.WorkerPhase{phase},
					},
					&meta.ListOptions{},
				)
				if err != nil {
					log.Fatal(err)
				}

				allWorkersByPhase.With(
					prometheus.Labels{"workerPhase": string(phase)},
				).Set(float64(len(eventsList.Items) +
					int(eventsList.RemainingItemCount)))

				// brigade_all_running_workers_duration

				var jobsList []core.Job
				for _, worker := range eventsList.Items {
					if phase == core.WorkerPhaseRunning {
						allRunningWorkersDuration.With(
							prometheus.Labels{"worker": worker.ID},
						).Set(time.Since(*worker.Worker.Status.Started).Seconds())
						// brigade_pending_jobs_total
						for _, job := range worker.Worker.Jobs {
							if job.Status.Phase == core.JobPhasePending {
								jobsList = append(jobsList, job)
							}
						}
					} else {
						allRunningWorkersDuration.Delete(
							prometheus.Labels{"worker": worker.ID},
						)
					}
				}

				// brigade_pending_jobs_total
				totalPendingJobs.Set(float64(len(jobsList)))
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

			// brigade_service_accounts_total
			saList, err := client.Authn().ServiceAccounts().List(
				context.Background(),
				&authn.ServiceAccountsSelector{},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			totalServiceAccounts.Set(float64(len(saList.Items) +
				int(saList.RemainingItemCount)))

			// brigade_projects_total
			projectList, err := client.Core().Projects().List(
				context.Background(),
				&core.ProjectsSelector{},
				&meta.ListOptions{},
			)
			if err != nil {
				log.Fatal(err)
			}

			totalProjects.Set(float64(len(projectList.Items) +
				int(projectList.RemainingItemCount)))

			time.Sleep(time.Duration(scrapeInterval) * time.Second)
		}
	}()
}

func initializeClient() (sdk.APIClient, error) {
	// The address of the Brigade 2 API server
	// beginning with http:// or https//
	apiAddress, err := os.GetRequiredEnvVar("API_ADDRESS")
	if err != nil {
		return nil, err
	}

	// An API token obtained using the Brigade 2 CLI
	apiToken, err := os.GetRequiredEnvVar("API_TOKEN")
	if err != nil {
		return nil, err
	}

	// Boolean indicating whether or not to ignore SSL errors
	apiIgnoreCertWarnings, err :=
		os.GetBoolFromEnvVar("API_IGNORE_CERT_WARNINGS", true)
	if err != nil {
		return nil, err
	}

	// Instantiate the API Client
	client := sdk.NewAPIClient(
		apiAddress,
		apiToken,
		&restmachinery.APIClientOptions{
			AllowInsecureConnections: apiIgnoreCertWarnings,
		},
	)

	return client, nil
}

func main() {
	scrapeInterval, err := os.GetIntFromEnvVar("PROM_SCRAPE_INTERVAL", 5)
	if err != nil {
		log.Fatal(err)
	}

	client, err := initializeClient()
	if err != nil {
		log.Fatal(err)
	}

	recordMetrics(client, scrapeInterval)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
