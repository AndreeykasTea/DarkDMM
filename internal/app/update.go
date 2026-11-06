package app

import (
	"sdmm/internal/app/selfupdate"

	"github.com/rs/zerolog/log"
)

var remoteManifest selfupdate.Manifest

// DarkDMM is a fork of the StrongDMM. Self update from the upstream repository
// would replace it with the original editor, so it's disabled completely.
func (a *app) checkForUpdatesV(bool) {
	log.Print("self updates are disabled in this build")
}

func (a *app) selfUpdate() {
	log.Print("self updates are disabled in this build")
}
