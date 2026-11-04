//go:build !unix

package main

func isNotADirectory(error) bool { return false }
