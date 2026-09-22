package service

import (
	"sync"
	"time"
)

// PersistentStorage saves metrics to durable storage.
type PersistentStorage interface {
	// SaveToFile writes metrics to the provided path.
	SaveToFile(path string) error
}

type persistence struct {
	storage     PersistentStorage
	path        string
	handleError func(error)
	mu          sync.Mutex
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
	err         error
}

func configurePersistence(service *MetricsService, storage PersistentStorage, path string, interval time.Duration, handleError func(error)) {
	p := &persistence{storage: storage, path: path, handleError: handleError, stop: make(chan struct{}), done: make(chan struct{})}
	service.persistence = p
	if interval == 0 {
		service.SetSaveOnUpdate(p.saveAndHandle)
		close(p.done)
		return
	}
	go p.run(interval)
}

func (p *persistence) save() error {
	// Serialize snapshots and renames, including synchronous handler saves.
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.storage.SaveToFile(p.path)
}

func (p *persistence) saveAndHandle() error {
	err := p.save()
	if err != nil && p.handleError != nil {
		p.handleError(err)
	}
	return err
}

func (p *persistence) run(interval time.Duration) {
	defer close(p.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.saveAndHandle()
		}
	}
}

func (p *persistence) close() error {
	p.once.Do(func() {
		close(p.stop)
		<-p.done
		p.err = p.save()
	})
	return p.err
}
