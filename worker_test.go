package worker

import (
	"fmt"
	"testing"
	"time"
)

type TaskMaster struct {
	*Master
	childs TaskSlaves
}

type TaskSlaves []*TaskSlave

type TaskSlave struct {
	Slave
}

func TestMasterCancel(t *testing.T) {

	//basic test
	{
		master := NewMaster()
		slave0 := NewSlave(master)

		sig_done_master := master.RunAsync(func(mc MasterCtx) error {
			select {
			case <-mc.SigCancel():
				return ErrCancel
			}
		})

		sig_done_slave0 := slave0.RunAsync(func(sc SlaveCtx) error {
			select {
			case <-sc.SigCancel():
				return ErrCancel
			}
		})

		master.Cancel()

		timout := time.NewTimer(time.Second * 2)
		for i, sig_done := range []chan error{sig_done_slave0, sig_done_master} {
			select {
			case <-sig_done:
				t.Logf("close %v", i)
			case <-timout.C:
				t.Fatal(fmt.Errorf("close channel does not shutdown workers"))
			}
		}
	}

	//basic test close slave before master
	{
		master := NewMaster()
		slave0 := NewSlave(master)

		sig_done_master := master.RunAsync(func(mc MasterCtx) error {
			select {
			case <-mc.SigCancel():
				return ErrCancel
			}
		})

		sig_done_slave0 := slave0.RunAsync(func(sc SlaveCtx) error {
			select {
			case <-sc.SigCancel():
				return ErrCancel
			}
		})

		slave0.Cancel()

		timeout := time.NewTimer(time.Second * 2)
		select {
		case <-sig_done_slave0:
			t.Logf("close")
		case <-timeout.C:
			t.Fatal(fmt.Errorf("close channel does not shutdown workers"))
		}

		master.Cancel()

		timeout = time.NewTimer(time.Second * 2)
		select {
		case <-sig_done_master:
			t.Logf("close")
		case <-timeout.C:
			t.Fatal(fmt.Errorf("close channel does not shutdown workers"))
		}
	}
}

func TestWorkerA(t *testing.T) {
	m := new(TaskMaster)
	m.Master = NewMaster()

	s1 := NewSlave(m.Master)
	s2 := NewSlave(m.Master)
	_, _ = s1, s2

	fn_task_0 := func(m MasterCtx) error {
		for {
			select {
			case <-m.SigCancel():
				return ErrCancel
			default:
			}

			time.Sleep(time.Second * 2)

			if true {
				return fmt.Errorf("bad error")
			}
		}
	}

	fn_task_1 := func(m MasterCtx) error {
		for {
			select {
			case <-m.SigCancel():
				return ErrCancel
			default:
			}

			time.Sleep(time.Second * 2)

			if true {
				return nil //success
			}
		}
	}

	fn_task_2 := func(m MasterCtx) error {
		select {
		case <-m.SigCancel():
			return ErrCancel
		}
	}

	_, _, _ = fn_task_0, fn_task_1, fn_task_2

	fn_tasks := []func(MasterCtx) error{fn_task_0, fn_task_1, fn_task_2}

	for i := 0; i < len(fn_tasks); i++ {

		sig_done := m.Master.RunAsync(fn_tasks[i])
		s1.RunAsync(func(sc SlaveCtx) error {
			t.Logf("s2 run")
			select {
			case <-sc.SigCancel():
				t.Logf("s2 cancel")
				return ErrCancel
			}
		})

		s2.RunAsync(func(sc SlaveCtx) error {
			t.Logf("s2 run")
			select {
			case <-sc.SigCancel():
				t.Logf("s2 cancel")
				return ErrCancel
			}
		})

		if i == 0 {
			err := <-sig_done
			if err != nil {
				t.Logf("0 err: %s", err.Error())
			} else {
				t.Log("0 success")
			}
		} else if i == 1 {
			err := <-sig_done
			if err != nil {
				t.Logf("1 err: %s", err.Error())
			} else {
				t.Log("1 success")
			}
		} else if i == 2 {
			//task waits forever for sig_cancel
			//restart worker with RunAsync
			time.Sleep(time.Second)
			m.Master.RunAsync(fn_tasks[i])

			//lets restart multiple times
			m.Master.RunAsync(fn_tasks[i])
			m.Master.RunAsync(fn_tasks[i])
			m.Master.RunAsync(fn_tasks[i])

			//note: still sig_done of first RunAsync call
			err := <-sig_done
			if err != nil {
				t.Logf("2 err: %s", err.Error())
			} else {
				t.Log("2 success")
			}
		}
	}
}
