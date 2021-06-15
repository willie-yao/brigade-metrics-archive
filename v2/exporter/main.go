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
	//"github.com/willie-yao/brigade-prometheus/v2/exporter/internal/os"
)

var (
	runningJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_running_jobs_total",
		Help: "The total number of processed events",
	})

	totalPendingWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "brigade_pending_workers_total",
		Help: "The total number of pending workers",
	})

	// pendingWorkersByProject = promauto.NewGaugeVec(prometheus.GaugeOpts{
	// 	Name: "brigade_pending_workers_by_project",
	// 	Help: "The number of pending workers by project",
	// }, []string{"projectID"})
)

func recordMetrics(client sdk.APIClient) {
	go func() {
		for {
			tempRunningJobs, err :=
				client.Core().Substrate().CountRunningJobs(context.Background())
			if err != nil {
				log.Println(err)
			}
			runningJobs.Set(float64(tempRunningJobs.Count))

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

			// Initialize vec
			// for projectID, workerList := range workerMapByProjectID {
			// 	pendingWorkersByProject.With(
			// 		prometheus.Labels{"projectID": projectID},
			// 	).Set(float64(len(workerList)))
			// }

			time.Sleep(2 * time.Second)
		}
	}()
}

func main() {

	// TODO: Uncomment these env variable declarations

	// The address of the Brigade 2 API server
	// beginning with http:// or https//
	// apiAddress, err := os.GetRequiredEnvVar("API_ADDRESS")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// An API token obtained using the Brigade 2 CLI
	// apiToken, err := os.GetRequiredEnvVar("API_TOKEN")
	// if err != nil {
	// 	log.Println(err)
	// }

	// Boolean indicating whether or not to ignore SSL errors
	// certWarning, err := os.GetBoolFromEnvVar("API_IGNORE_CERT_WARNINGS", true)
	// if err != nil {
	// 	log.Println(err)
	// }

	// Instantiate the API Client
	client := sdk.NewAPIClient(
		"https://localhost:8443",
		"Placeholder",
		&restmachinery.APIClientOptions{
			AllowInsecureConnections: true,
		},
	)

	recordMetrics(client)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
