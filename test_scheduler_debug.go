package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func main() {
	store, err := cache.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created job: %s, status: %s\n", job.ID, job.Status)

	pastTime := time.Now().Add(-10 * time.Minute)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Paused job\n")

	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paused jobs count: %d\n", len(pausedJobs))
	if len(pausedJobs) > 0 {
		fmt.Printf("First paused job status: %s\n", pausedJobs[0].Status)
	}

	if err := store.ResumeSyncJobByID(job.ID); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resumed job\n")

	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Paused jobs count after resume: %d\n", len(pausedJobs))

	resumedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Resumed job status: %s\n", resumedJob.Status)
}
