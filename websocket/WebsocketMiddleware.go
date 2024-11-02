package websocket

import (
	"errors"
	"fmt"
	websocket2 "github.com/fasthttp/websocket"
	"github.com/linxlib/fw"
	"github.com/valyala/fasthttp"
	"log"
)

var _ fw.IMiddlewareMethod = (*WebsocketMiddleware)(nil)

const websocketAttr = "Websocket"
const websocketName = "Websocket"

func NewWebsocketMiddleware() fw.IMiddlewareMethod {
	mw := &WebsocketMiddleware{
		MiddlewareMethod: fw.NewMiddlewareMethod(websocketName, websocketAttr),
		upgrade: websocket2.FastHTTPUpgrader{
			CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
				return true
			},
		},
	}

	return mw
}

// WebsocketMiddleware used for simple websocket communication with server
type WebsocketMiddleware struct {
	*fw.MiddlewareMethod
	upgrade websocket2.FastHTTPUpgrader
}

func (w *WebsocketMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		fmt.Println(ctx.ControllerName, ctx.MethodName)
		err := w.upgrade.Upgrade(context.GetFastContext(), func(ws *websocket2.Conn) {
			defer ws.Close()
			for {
				mt, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					break
				}
				log.Printf("recv: %s", message)
				//这里拿到了ws的数据 需要去调用下一级的方法了
				//将参数数据Map一下，在下一级调用时会自动映射进去
				context.Map(message)
				ctx.Next(context)
				// 调用完实际的方法之后，需要把该回写的数据拿过来处理了
				result, ex1 := context.Get("fw_data")
				resultErr, ex2 := context.Get("fw_err")
				if ex2 && resultErr != nil {
					err = ws.WriteMessage(mt, []byte(resultErr.(error).Error()))
					if err != nil {
						//log.Println("write:", err)
						break
					}
				} else {
					if ex1 {
						err = ws.WriteMessage(mt, result.([]byte))
						if err != nil {
							log.Println("write:", err)
							break
						}
					}

				}

			}

		})
		if err != nil {
			var handshakeError websocket2.HandshakeError
			if errors.As(err, &handshakeError) {
				log.Println(err)
			}
			return
		}
	}
}
