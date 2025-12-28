package ui

import (
	"fmt"
	"os"
)

type Verbosity int

const (
	VerbosityQuiet Verbosity = iota
	VerbosityNormal
	VerbosityVerbose
)

var currentVerbosity = VerbosityNormal

func SetVerbosity(v Verbosity) {
	currentVerbosity = v
}

func GetVerbosity() Verbosity {
	return currentVerbosity
}

func IsQuiet() bool {
	return currentVerbosity == VerbosityQuiet
}

func IsVerbose() bool {
	return currentVerbosity == VerbosityVerbose
}

func Print(format string, args ...any) {
	if currentVerbosity >= VerbosityNormal {
		fmt.Printf(format, args...)
	}
}

func Println(args ...any) {
	if currentVerbosity >= VerbosityNormal {
		fmt.Println(args...)
	}
}

func Debug(format string, args ...any) {
	if currentVerbosity >= VerbosityVerbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
