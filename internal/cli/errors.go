package cli

import "errors"

var errOutput = errors.New("could not write command output")

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	if errors.Is(err, errOutput) {
		return 1
	}

	return 2
}
