package vue

import (
	"bytes"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/valyala/fasthttp"
	"strings"
)

type PageOptions struct {
	DirPath   string `yaml:"dir_path" default:"./dist"`       // 资源目录
	RoutePath string `yaml:"route_path" default:"/"`          // 路由
	TryFiles  string `yaml:"try_files" default:"/index.html"` // try_files 兼容nginx的try_files
	Base      string `yaml:"base" default:""`                 //打包时的base (例如vite的base, 即前端打包时的根路径) 例如 /web
}

func NewPageMiddleware(dir ...string) *PageMiddleware {
	pm := &PageMiddleware{
		options:          &PageOptions{},
		MiddlewareGlobal: fw.NewMiddlewareGlobal("Page"),
	}
	if len(dir) > 0 {
		pm.options.DirPath = dir[0]
	}
	return pm
}

// PageMiddleware 页面中间件
type PageMiddleware struct {
	*fw.MiddlewareGlobal
	options *PageOptions
}

func (p *PageMiddleware) DoInitOnce() {
	p.LoadConfig("page", p.options)
}

func stripLeadingSlashes(path []byte, stripSlashes int) []byte {
	for stripSlashes > 0 && len(path) > 0 {
		if path[0] != '/' {
			// developer sanity-check
			panic("BUG: path must start with slash")
		}
		n := bytes.IndexByte(path[1:], '/')
		if n < 0 {
			path = path[:0]
			break
		}
		path = path[n+1:]
		stripSlashes--
	}
	return path
}

func FSHandler(root string, tryFiles string, base string) fw.HandlerFunc {
	fs := &fasthttp.FS{
		Root:               root,
		IndexNames:         []string{"index.html", "index.htm"},
		GenerateIndexPages: false,
		AcceptByteRange:    true,
	}
	var normalExt = func(uri []byte) bool {
		exts := []string{".html", ".css", ".js", ".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".ttf", ".eot", ".woff", ".woff2"}
		for _, ext := range exts {
			if strings.HasSuffix(conv.String(uri), ext) {
				return true
			}
		}
		return false

	}
	fs.PathRewrite = func(ctx *fasthttp.RequestCtx) []byte {
		var ru []byte
		if base != "" {
			//TODO: determine stripSlashes count 根据base和route_path判断要去掉几层
			ru = stripLeadingSlashes(ctx.RequestURI(), 1)
		} else {
			ru = ctx.RequestURI()
		}
		if normalExt(ru) ||
			conv.String(ru) == "/" ||
			conv.String(ru) == "" {
			return ru
		} else {
			return conv.Bytes(tryFiles)
		}
	}
	return func(context *fw.Context) {
		fs.NewRequestHandler()(context.GetFastContext())
	}
}

func (p *PageMiddleware) Router(ctx *fw.MiddlewareContext) []*fw.RouteItem {

	return []*fw.RouteItem{
		{
			Method:           "GET",
			Path:             p.options.RoutePath + "{any:*}",
			IsHide:           false,
			H:                FSHandler(p.options.DirPath, p.options.TryFiles, p.options.Base),
			Middleware:       p,
			OverrideBasePath: true,
		},
	}

}
