package service

import "time"

// PersistentStorage saves metrics to durable storage.
type PersistentStorage interface {
	// SaveToFile writes metrics to the provided path.
	SaveToFile(path string) error
}

func configurePersistence(service *MetricsService, storage PersistentStorage, path string, interval time.Duration, handleError func(error)) {
	save := func() {
		if err := storage.SaveToFile(path); err != nil && handleError != nil {
			handleError(err)
		}
	}

	if interval == 0 {
		service.SetSaveOnUpdate(save)
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			save()
		}
	}()
}
