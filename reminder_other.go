//go:build !windows

package main

import "errors"

func (a *App) startReminders() (func(), func(bool) error, error) {
	return nil, nil, errors.New("corner reminder windows are supported only on Windows")
}
