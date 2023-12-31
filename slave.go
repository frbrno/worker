package worker

import "sync"

func NewSlave(m *Master, once bool) *Slave {
	s := &Slave{
		m:          m,
		wg:         &sync.WaitGroup{},
		sig_cancel: make(chan struct{}),
		is_active:  false,
		once:       once,
	}
	m.mu.Lock()
	m.slaves = append(m.slaves, s)
	m.mu.Unlock()
	return s
}

type Slave struct {
	m          *Master
	wg         *sync.WaitGroup
	sig_cancel chan struct{}
	once       bool

	is_active bool
}

type SlaveCtx interface {
	SigCancel() chan struct{}
}

type slave_ctx struct {
	s          *Slave
	sig_cancel chan struct{}
}

func (sc *slave_ctx) SigCancel() chan struct{} {
	return sc.sig_cancel
}

// Cancel, stops the goroutine started by RunAsync
func (s *Slave) Cancel() {
	<-s.m.sig_mu
	defer func() { s.m.sig_mu <- struct{}{} }()

	s.m.mu.Lock()
	if s.is_active {
		close(s.sig_cancel)
	}
	s.is_active = false

	if s.once {
		s.m.removeSlave(s)
	}

	s.m.mu.Unlock()

	s.wg.Wait()
}

func (s *Slave) RunAsync(fn func(SlaveCtx) error) chan error {
	<-s.m.sig_mu
	defer func() { s.m.sig_mu <- struct{}{} }()

	sig_done := make(chan error, 1)

	s.m.mu.Lock()
	if len(s.m.errs) > 0 {
		sig_done <- s.m.errs[0]
		s.m.mu.Unlock()
		return sig_done
	}

	if !s.m.is_active {
		sig_done <- ErrMasterNotActive
		s.m.mu.Unlock()
		return sig_done
	}

	if s.is_active {
		close(s.sig_cancel)
	}
	s.is_active = true

	s.m.mu.Unlock()

	s.wg.Wait()

	s.wg = &sync.WaitGroup{}

	sig_cancel := make(chan struct{})
	s.sig_cancel = sig_cancel

	s.m.wg.Add(1)
	s.wg.Add(1)
	go func(sig_cancel chan struct{}) {
		defer s.m.wg.Done()
		defer s.wg.Done()

		err := fn(&slave_ctx{
			s:          s,
			sig_cancel: sig_cancel,
		})

		//TODO: if err != nil is bad set error close all slaves

		select {
		case <-s.m.sig_mu:
			defer func() { s.m.sig_mu <- struct{}{} }()
			s.m.mu.Lock()
			s.is_active = false
			if s.once {
				s.m.removeSlave(s)
			}
			s.m.mu.Unlock()
		case <-sig_cancel:
			// next RunAsync call holds lock and wants cancel current goroutine
		}

		sig_done <- err

	}(sig_cancel)

	return sig_done
}
