package webui

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

//go:embed all:dist
var embeddedDist embed.FS

// Register mounts the embedded Quasar SPA on NoRoute (history mode: unknown paths → index.html).
// Register after API routes and /swagger so only non-API paths hit the SPA.
func Register(h *server.Hertz) {
	sub, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		panic("webui: " + err.Error())
	}
	h.NoRoute(func(ctx context.Context, c *app.RequestContext) {
		if string(c.Request.Method()) != consts.MethodGet && string(c.Request.Method()) != consts.MethodHead {
			c.AbortWithStatus(consts.StatusMethodNotAllowed)
			return
		}
		adaptor.HertzHandler(http.FileServer(http.FS(fallbackFS{sub})))(ctx, c)
	})
}

// fallbackFS serves real files from embed; missing paths fall back to index.html for Vue Router.
type fallbackFS struct {
	fs fs.FS
}

func (f fallbackFS) Open(name string) (fs.File, error) {
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = "index.html"
	}
	file, err := f.fs.Open(name)
	if err != nil {
		if name == "index.html" {
			return nil, err
		}
		return f.fs.Open("index.html")
	}
	return file, nil
}
