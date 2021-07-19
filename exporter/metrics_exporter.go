package main

import (
	"context"
	"log"
	"time"

	"github.com/brigadecore/brigade/sdk/v2"
	"github.com/brigadecore/brigade/sdk/v2/authn"
	"github.com/brigadecore/brigade/sdk/v2/core"
	"github.com/brigadecore/brigade/sdk/v2/meta"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metricsExporter struct {
	apiClient                 sdk.APIClient
	scrapeInterval            time.Duration
	totalRunningJobs          prometheus.Gauge
	totalPendingJobs          prometheus.Gauge
	allWorkersByPhase         *prometheus.GaugeVec
	allRunningWorkersDuration *prometheus.GaugeVec
	totalUsers                prometheus.Gauge
	totalServiceAccounts      prometheus.Gauge
	totalProjects             prometheus.Gauge
}

func newMetricsExporter(
	apiClient sdk.APIClient,
	scrapeInterval time.Duration,
) *metricsExporter {
	return &metricsExporter{
		apiClient:      apiClient,
		scrapeInterval: scrapeInterval,
		totalRunningJobs: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "brigade_running_jobs_total",
				Help: "The total number of running jobs",
			},
		),
		totalPendingJobs: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "brigade_pending_jobs_total",
				Help: "The total number of pending jobs",
			},
		),
		allWorkersByPhase: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "brigade_all_workers_by_phase",
				Help: "All workers separated by phase",
			},
			[]string{"workerPhase"},
		),
		allRunningWorkersDuration: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "brigade_all_running_workers_duration",
				Help: "The duration of all running workers",
			},
			[]string{"worker"},
		),
		totalUsers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "brigade_users_total",
				Help: "The total number of users",
			},
		),
		totalServiceAccounts: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "brigade_service_accounts_total",
				Help: "The total number of service accounts",
			},
		),
		totalProjects: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "brigade_projects_total",
				Help: "The total number of brigade projects",
			},
		),
	}
}

func (m *metricsExporter) run(ctx context.Context) {
	ticker := time.NewTicker(m.scrapeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.recordMetrics()
		case <-ctx.Done():
			return
		}
	}
}

func (m *metricsExporter) recordMetrics() {
	// brigade_running_jobs_total
	tempRunningJobs, err :=
		m.apiClient.Core().Substrate().CountRunningJobs(context.Background())
	if err != nil {
		log.Println(err)
	}
	m.totalRunningJobs.Set(float64(tempRunningJobs.Count))

	for _, phase := range core.WorkerPhasesAll() {
		// brigade_all_workers_by_phase
		var eventList core.EventList
		eventList, err = m.apiClient.Core().Events().List(
			context.Background(),
			&core.EventsSelector{
				WorkerPhases: []core.WorkerPhase{phase},
			},
			&meta.ListOptions{},
		)
		if err != nil {
			log.Fatal(err)
		}

		m.allWorkersByPhase.With(
			prometheus.Labels{"workerPhase": string(phase)},
		).Set(
			float64(len(eventList.Items) + int(eventList.RemainingItemCount)),
		)

		// brigade_all_running_workers_duration

		var jobsList []core.Job
		for _, worker := range eventList.Items {
			if phase == core.WorkerPhaseRunning {
				m.allRunningWorkersDuration.With(
					prometheus.Labels{"worker": worker.ID},
				).Set(
					time.Since(*worker.Worker.Status.Started).Seconds(),
				)
				// brigade_pending_jobs_total
				for _, job := range worker.Worker.Jobs {
					if job.Status.Phase == core.JobPhasePending {
						jobsList = append(jobsList, job)
					}
				}
			}
		}

		// brigade_pending_jobs_total
		m.totalPendingJobs.Set(float64(len(jobsList)))
	}

	// brigade_users_total
	userList, err := m.apiClient.Authn().Users().List(
		context.Background(),
		&authn.UsersSelector{},
		&meta.ListOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}

	m.totalUsers.Set(float64(len(userList.Items) +
		int(userList.RemainingItemCount)))

	// brigade_service_accounts_total
	saList, err := m.apiClient.Authn().ServiceAccounts().List(
		context.Background(),
		&authn.ServiceAccountsSelector{},
		&meta.ListOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}

	m.totalServiceAccounts.Set(
		float64(len(saList.Items) + int(saList.RemainingItemCount)),
	)

	// brigade_projects_total
	projectList, err := m.apiClient.Core().Projects().List(
		context.Background(),
		&core.ProjectsSelector{},
		&meta.ListOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}

	m.totalProjects.Set(float64(len(projectList.Items) +
		int(projectList.RemainingItemCount)))
}
