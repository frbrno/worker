package worker

import (
	"errors"
)

var ErrCancel = errors.New("cancel")
var ErrMasterNotActive = errors.New("master not active")

type SigCancel chan struct{}
