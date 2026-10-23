package watch

import "martinshaw.co/environment-manager/internal/store"

// DecideDrift reports whether a disk hash should raise or clear a drift notice.
// latest is empty when the project has no selected version. disk is
// store.MissingHash when .env is absent.
func DecideDrift(latest, disk, notified string) (notify, clear bool) {
	drifted := latest != "" && disk != latest
	unsaved := latest == "" && disk != "" && disk != store.MissingHash
	if !drifted && !unsaved {
		return false, notified != ""
	}
	if notified == disk {
		return false, false
	}
	return true, false
}

// DecideIgnore reports whether a gitignore status should raise or clear a notice.
func DecideIgnore(statusKey, notified string, attention bool) (notify, clear bool) {
	if !attention {
		return false, notified != ""
	}
	if statusKey == notified {
		return false, false
	}
	return true, false
}
