package domain

import "errors"

var (
	ErrExecutorOffline = errors.New("scheduler executor offline")
)
