package parexec

import "sync"

// We have errgroup at home

type ParExecutor struct {
	sem     chan struct{}
	wg      sync.WaitGroup
	errChan chan error
	errOnce sync.Once
}

func NewParExecutor(workers int) *ParExecutor {
	return &ParExecutor{
		sem:     make(chan struct{}, workers),
		errChan: make(chan error, 1),
	}
}

func (e *ParExecutor) Go(fn func() error) {
	e.wg.Add(1)
	go func() {
		e.sem <- struct{}{}
		defer func() {
			<-e.sem
			e.wg.Done()
		}()
		if err := fn(); err != nil {
			e.errOnce.Do(func() {
				e.errChan <- err
			})
		}
	}()
}

func (e *ParExecutor) Wait() error {
	e.wg.Wait()
	close(e.errChan)
	return <-e.errChan
}
