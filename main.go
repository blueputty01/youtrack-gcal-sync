package main

import (
	"github.com/blueputty01/task-sync/internal/sync"
	"log/slog"
)

func main() {
	_, err := sync.NewSynchronizer("sync.db")
	if err != nil {
		slog.Error("Failed to create synchronizer:", "DB error", err)
	}

	//synchronizer.Sync
}
