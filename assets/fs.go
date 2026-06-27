package assets

import (
	"embed"
	"io/fs"

	"github.com/samber/lo"
)

//go:embed migrations
var mFS embed.FS

func Migrations() fs.FS {
	return lo.Must(fs.Sub(mFS, "migrations"))
}

//go:embed html
var htmlFS embed.FS

func HTML() fs.FS {
	return lo.Must(fs.Sub(htmlFS, "html"))
}

//go:embed configs
var cFS embed.FS

func SharedConfigs() fs.FS {
	return lo.Must(fs.Sub(cFS, "configs"))
}
