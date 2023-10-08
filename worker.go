package worker

import (
	"errors"
)

var ErrCancel = errors.New("cancel")

type SigCancel chan struct{}
