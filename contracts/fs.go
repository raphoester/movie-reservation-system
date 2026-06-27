package contracts

import (
	"embed"
	"io/fs"

	"github.com/samber/lo"
)

//go:embed oapi
var mFS embed.FS

func OAPI() fs.FS {
	return lo.Must(fs.Sub(mFS, "oapi"))
}
