package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	p := NewPool()

	w1 := NewWorker(p)
	w2 := NewWorker(p)

	w1.Run(context.Background(), func(ctx context.Context) (err error) {
		t.Log("hello w1")
		<-ctx.Done()
		t.Log("bye w1")
		return ErrCancel
	})

	w2.Run(context.Background(), func(ctx context.Context) (err error) {
		t.Log("hello w2")

		err = errors.Join(ErrFatal, errors.New("something bad happend"))
		return err

		<-ctx.Done()
		t.Log("bye w2")
		return ErrCancel
	})

	go func() {
		time.Sleep(time.Second * 2)
		p.Shutdown()
	}()

	err := <-p.SigDone()
	if err != nil {
		t.Logf("bye err: %s", err.Error())
	}
	t.Log("bye")
}
