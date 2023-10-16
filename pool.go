package worker

import (
	"errors"
	"sync"
)

func NewPool() *Pool {
	p := &Pool{
		mu:       &sync.RWMutex{},
		wg:       &sync.WaitGroup{},
		err:      nil,
		workers:  []*W{},
		sig_done: make(chan error, 1),
		sig_mu:   make(chan struct{}, 1),
	}
	p.sig_mu <- struct{}{}
	return p
}

type Pool struct {
	mu      *sync.RWMutex
	wg      *sync.WaitGroup
	err     error
	workers []*W

	sig_done SigDone
	sig_mu   chan struct{}
}

func (p *Pool) handle_err(err error) {
	if err != nil && errors.Is(err, ErrFatal) {
		p.shutdown(err)
	}
}

func (p *Pool) SigDone() SigDone {
	return p.sig_done
}

func (p *Pool) shutdown(err error) {
	p.mu.Lock()
	if p.err != nil { //already shutting down
		p.mu.Unlock()
		return
	}
	p.err = err
	p.mu.Unlock()

	for _, worker := range p.workers {
		worker.fn_cancel()
	}
	p.wg.Wait()

	p.mu.Lock()
	p.wg = new(sync.WaitGroup)
	p.mu.Unlock()

	p.sig_done <- err
}

func (p *Pool) Shutdown() {
	<-p.sig_mu
	defer func() { p.sig_mu <- struct{}{} }()

	p.shutdown(ErrCancel)
}
