//go:build windows

package uping

func checkPrivileged() (bool, error) {
	return true, nil
}
