package worker

import (
	"context"
	"errors"
)

var ErrFatal = errors.New("is fatal")
var ErrCancel = errors.New("cancel")
var ErrMasterNotActive = errors.New("master not active")

type SigCancel chan struct{}
type SigDone chan error

type Task func(ctx context.Context) (err error)

type Worker interface {
	Pool() *Pool
	Cancel()
	Run(context.Context, Task) SigDone
}

func NewWorker(p *Pool) Worker {
	w := &W{
		p:               p,
		fn_cancel:       func() {},
		sig_cancel_done: make(chan struct{}),
		once:            false,
		is_active:       false,
	}
	close(w.sig_cancel_done)

	w.p.mu.Lock()
	p.workers = append(p.workers, w)
	w.p.mu.Unlock()

	return w
}

type W struct {
	p *Pool

	fn_cancel       func()
	sig_cancel_done chan struct{}
	once            bool

	is_active bool
}

func (w *W) Pool() *Pool {
	return w.p
}

func (w *W) Run(ctx context.Context, task Task) SigDone {
	<-w.p.sig_mu
	defer func() { w.p.sig_mu <- struct{}{} }()

	sig_done := make(SigDone, 1)

	w.p.mu.Lock()
	if w.p.err != nil {
		sig_done <- w.p.err
		w.p.mu.Unlock()
		return sig_done
	}

	w.fn_cancel()
	w.p.mu.Unlock()

	<-w.sig_cancel_done

	w.p.mu.Lock()
	if w.p.err != nil {
		sig_done <- w.p.err
		w.p.mu.Unlock()
		return sig_done
	}

	ctx, fn_cancel := context.WithCancel(ctx)

	w.is_active = true

	w.fn_cancel = fn_cancel
	sig_cancel_done := make(chan struct{})
	w.sig_cancel_done = sig_cancel_done

	w.p.mu.Unlock()

	w.p.wg.Add(1)
	go func(ctx context.Context, sig_cancel_done chan struct{}, sig_done SigDone) {
		err := task(ctx)

		select {
		case <-w.p.sig_mu: //try get the lock if no one else holds it
			defer func() { w.p.sig_mu <- struct{}{} }()
		default:
		}

		sig_done <- err

		w.p.wg.Done()
		w.p.handle_err(err)
		close(sig_cancel_done)

	}(ctx, sig_cancel_done, sig_done)
	return sig_done
}

func (w *W) Cancel() {
	<-w.p.sig_mu
	defer func() { w.p.sig_mu <- struct{}{} }()

	w.fn_cancel()
	<-w.sig_cancel_done

	w.p.mu.Lock()
	w.is_active = false
	w.fn_cancel = func() {}
	w.p.mu.Unlock()
}
