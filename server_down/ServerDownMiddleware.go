package fw_middlewares

import (
	"github.com/linxlib/fw"
	"io"
	"net/http"
	"strings"
)

// ServerDownMiddleware is a middleware which provides an api to mark server down.
type ServerDownMiddleware struct {
	*fw.MiddlewareGlobal
	sdo        *ServerDownOption
	serverDown bool
}

type ServerDownOption struct {
	Method string `yaml:"method" default:"PATCH"`
	Path   string `yaml:"path" default:"/serverDown/{key}"`
	Key    string `yaml:"key" default:""`
}

func (s *ServerDownMiddleware) DoInitOnce() {
	s.LoadConfig("serverDown", s.sdo)
}

func (s *ServerDownMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		if s.serverDown {
			resp, _ := http.Get("https://shuye.dev/maintenance-page/")
			bs, _ := io.ReadAll(resp.Body)
			context.Data(200, "text/html", bs)
			return
		}

		ctx.Next(context)
	}
}

func (s *ServerDownMiddleware) Router(ctx *fw.MiddlewareContext) []*fw.RouteItem {
	if s.sdo.Key == "" {
		return nil
	}
	return []*fw.RouteItem{&fw.RouteItem{
		Method: "PATCH",
		Path:   "/serverDown/{key}",
		IsHide: true,
		H: func(context *fw.Context) {
			str := context.GetFastContext().UserValue("key").(string)
			str = strings.TrimSpace(str)
			if str == s.sdo.Key {
				s.serverDown = !s.serverDown
			}
			context.String(200, "ok")
		},
		Middleware: s,
	}}
}

const serverDownName = "ServerDown"

func NewServerDownMiddleware() fw.IMiddlewareGlobal {
	return &ServerDownMiddleware{
		sdo:              new(ServerDownOption),
		MiddlewareGlobal: fw.NewMiddlewareGlobal(serverDownName),
	}
}
