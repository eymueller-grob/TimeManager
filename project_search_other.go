//go:build !windows

package main

import "errors"

func (a *App) startProjectSearch() (func(), func() error, error) {
	return nil, nil, errors.New("tray project search is supported only on Windows")
}
