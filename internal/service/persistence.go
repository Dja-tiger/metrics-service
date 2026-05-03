package service

import "time"

type PersistentStorage interface {
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
