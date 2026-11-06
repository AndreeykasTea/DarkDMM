package dmmsave

import (
	"sdmm/internal/dmapi/dmenv"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

// Save saves the map to its file. Returns an error if the map wasn't saved.
func Save(dme *dmenv.Dme, dmm *dmmap.Dmm, cfg Config) error {
	return SaveV(dme, dmm, dmm.Path.Absolute, cfg)
}

func SaveV(dme *dmenv.Dme, dmm *dmmap.Dmm, path string, cfg Config) error {
	log.Printf("save started [%s]...", path)

	sp, err := makeSaveProcess(cfg, dme, dmm, path)
	if err != nil {
		log.Print("unable to start save process")
		util.ShowErrorDialog("Не удалось начать сохранение")
		return err
	}

	if cfg.SanitizeVariables {
		sp.sanitizeVariables()
	}

	sp.handleReusedKeys()
	if err = sp.handleLocationsWithoutKeys(); err != nil {
		log.Print("unable to handle locations without keys:", err)
		util.ShowErrorDialog("Не удалось сохранить карту: " + err.Error())
		return err
	}
	if err = sp.output.Save(); err != nil {
		log.Print("unable to write the map:", err)
		util.ShowErrorDialog("Не удалось записать карту: " + err.Error())
		return err
	}

	log.Print("save finished")
	return nil
}
