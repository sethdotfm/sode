//go:build !darwin && !linux

package main

func noEcho(int) func() { return func() {} }
