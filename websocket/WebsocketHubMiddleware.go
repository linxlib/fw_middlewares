package websocket

import (
	"errors"
	websocket2 "github.com/fasthttp/websocket"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/valyala/fasthttp"
	"log"
	"math/rand/v2"
)

var _ fw.IMiddlewareCtl = (*WebsocketHubMiddleware)(nil)

const websocketHubAttr = "WebsocketHub"
const websocketHubName = "WebsocketHub"

func NewWebsocketHubMiddleware() fw.IMiddlewareCtl {
	mw := &WebsocketHubMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl(websocketHubName, websocketHubAttr),
		upgrade: websocket2.FastHTTPUpgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
				return true
			},
		}, hub: newHub()}
	go mw.hub.run()
	return mw
}

// WebsocketHubMiddleware
// used for chat
type WebsocketHubMiddleware struct {
	*fw.MiddlewareCtl
	upgrade websocket2.FastHTTPUpgrader
	hub     *Hub
	route   string
	method  string
}

func (w *WebsocketHubMiddleware) Router(ctx *fw.MiddlewareContext) []*fw.RouteItem {
	w.route = ctx.GetParam("route")
	return []*fw.RouteItem{&fw.RouteItem{
		Method:     "ANY",
		Path:       w.route,
		Middleware: w,
		H: func(context *fw.Context) {
			// 需要将hub注入到controller层
			// context 是方法层的
			// 或者在这个controller下的所有方法的context都注入进去
			err := w.upgrade.Upgrade(context.GetFastContext(), func(ws *websocket2.Conn) {
				client := &Client{hub: w.hub, conn: ws, send: make(chan []byte, 256), ID: conv.String(rand.IntN(100))}
				client.hub.register <- client
				log.Println("register client")
				go client.writePump()
				m := []byte(client.ID + " 刚刚加入")

				go func() { client.hub.broadcast <- m }()
				client.readPump()
			})
			if err != nil {
				var handshakeError websocket2.HandshakeError
				if errors.As(err, &handshakeError) {
					log.Println(err)
				}
				return
			}
		},
	}}
}
func (w *WebsocketHubMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		context.Map(w.hub)
		ctx.Next(context)
	}
}
