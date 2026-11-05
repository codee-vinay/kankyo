package platform

// ComSpecMismatch is not applicable on non-Windows platforms.
func ComSpecMismatch() (mismatch bool, expected, actual string) {
	return false, "", ""
}
