package worker

import "sync"

func NewMaster() *Master {
	m := &Master{
		mu:         &sync.RWMutex{},
		wg:         &sync.WaitGroup{},
		sig_cancel: make(chan struct{}),
		errs:       []error{},
		slaves:     make([]*Slave, 0),
		mu_slaves:  &sync.RWMutex{},
		sig_mu:     make(chan struct{}, 1),
	}
	m.sig_mu <- struct{}{}

	return m
}

type MasterCtx interface {
	SigCancel() chan struct{}
}

type master_ctx struct {
	m          *Master
	sig_cancel chan struct{}
}

func (mc *master_ctx) SigCancel() chan struct{} {
	return mc.sig_cancel
}

type Master struct {
	mu         *sync.RWMutex
	wg         *sync.WaitGroup
	sig_cancel chan struct{}
	errs       []error
	slaves     []*Slave
	mu_slaves  *sync.RWMutex

	sig_mu chan struct{}

	is_active bool
}

// Cancel, stops the goroutine started by RunAsync and all its Slaves
func (m *Master) Cancel() {
	<-m.sig_mu
	defer func() { m.sig_mu <- struct{}{} }()

	m.mu.Lock()
	if m.is_active {
		close(m.sig_cancel)
	}
	for _, s := range m.slaves {
		if s.is_active {
			close(s.sig_cancel)
		}
	}
	m.mu.Unlock()

	m.wg.Wait()

	m.mu.Lock()
	m.errs = append(m.errs, ErrCancel)
	m.is_active = false
	for _, s := range m.slaves {
		s.is_active = false
	}
	m.mu.Unlock()
}

func (m *Master) removeSlave(slave *Slave) {
	for i, s := range m.slaves {
		if s == slave {
			m.slaves[i] = m.slaves[len(m.slaves)-1]
			m.slaves[len(m.slaves)-1] = nil
			m.slaves = m.slaves[:len(m.slaves)-1]
		}
	}
}

func (m *Master) RunAsync(fn func(MasterCtx) error) chan error {
	<-m.sig_mu
	defer func() { m.sig_mu <- struct{}{} }()

	sig_done := make(chan error, 1)

	m.mu.Lock()
	if len(m.errs) > 0 {
		sig_done <- m.errs[0]
		m.mu.Unlock()
		return sig_done
	}

	if m.is_active {
		close(m.sig_cancel)
	}
	m.is_active = true

	for _, s := range m.slaves {
		if s.is_active {
			close(s.sig_cancel)
			//could still be active, but need to set it here since sig_mu is locked
			s.is_active = false
		}
	}
	m.mu.Unlock()

	m.wg.Wait()

	m.wg = &sync.WaitGroup{}

	sig_cancel := make(chan struct{})
	m.sig_cancel = sig_cancel

	m.wg.Add(1)
	go func(sig_cancel chan struct{}) {

		defer m.wg.Done()
		err := fn(&master_ctx{
			m:          &Master{},
			sig_cancel: sig_cancel,
		})

		//TODO: if err != nil is bad set error close all slaves

		select {
		case <-m.sig_mu:
			defer func() { m.sig_mu <- struct{}{} }()
			m.mu.Lock()
			m.is_active = false
			m.mu.Unlock()
		case <-sig_cancel:
			// next RunAsync call holds lock and wants cancel current goroutine
		}

		sig_done <- err

	}(sig_cancel)

	return sig_done
}
