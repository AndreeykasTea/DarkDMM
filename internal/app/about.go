package app

import (
	"fmt"

	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/env"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/rsc"

	"github.com/rs/zerolog/log"
	"github.com/skratchdot/open-golang/open"
)

var aboutText string

func (a *app) openAboutWindow() {
	if len(aboutText) == 0 {
		aboutText = rsc.AboutTxt(env.Title + " " + env.Version)
	}
	dialog.Open(dialog.TypeCustom{
		Title:       "О программе",
		CloseButton: true,
		Layout: w.Layout{
			w.Text(aboutText),
			w.NewLine(),
			w.AlignTextToFramePadding(),
			w.Text("Ревизия:"),
			w.SameLine(),
			w.Button(env.Revision, func() {
				link := fmt.Sprintf("%s/tree/%s", env.GitHub, env.Revision)
				log.Print("do open revision link:", link)
				if err := open.Run(link); err != nil {
					log.Print("unable to open revision link:", err)
				}
			}).Small(true),
		},
	})
}
