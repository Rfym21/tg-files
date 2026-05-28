package web

import "embed"

//go:embed all:dist
var embeddedDist embed.FS

func init() {
	distFS = embeddedDist
}
