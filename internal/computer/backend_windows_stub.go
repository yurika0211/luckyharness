//go:build !windows

package computer

import "fmt"

// NewWindowsBackend keeps backend selection cross-compilable. The concrete
// implementation is only available in Windows builds.
func NewWindowsBackend() (Backend, error) {
	return nil, fmt.Errorf("computer: windows backend requires a Windows build")
}
