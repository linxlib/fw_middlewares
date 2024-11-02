package log

import (
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/types"
	"time"
)

const (
	loggerName = "Logger"
)

var _ fw.IMiddlewareGlobal = (*LoggerMiddleware)(nil)

func NewLoggerMiddleware() fw.IMiddlewareGlobal {
	return &LoggerMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(loggerName)}
}

type LoggerMiddleware struct {
	*fw.MiddlewareGlobal
	Logger types.ILogger `inject:""`
}

func (w *LoggerMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		fctx := context.GetFastContext()
		start := time.Now()
		params := &LogParams{}
		params.BodySize = len(fctx.PostBody())
		params.Path = conv.String(fctx.Request.RequestURI())
		//// add Cloudflare CDN real ip header support
		//if w.realIPHeader != "" {
		//	params.ClientIP = string(fctx.Request.Header.Peek(w.realIPHeader))
		//}
		params.ClientIP = fctx.RemoteIP().String()
		params.Protocol = conv.String(fctx.Request.Header.Protocol())
		params.UserAgent = conv.String(fctx.Request.Header.UserAgent())
		params.Method = conv.String(fctx.Method())
		ctx.Next(context)
		params.TimeStamp = time.Now()
		params.Latency = params.TimeStamp.Sub(start)
		params.StatusCode = fctx.Response.StatusCode()
		err, exist := context.Get("fw_err")
		if exist && err != nil {
			params.ErrorMessage = "\nErr:" + err.(error).Error()
		}
		info := make([]types.Arg, 0)
		k, v := params.TimeStampWithColor("%19s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		k, v = params.ClientIPWithColor("%13s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		info = append(info, types.Arg{
			Key:   `-`,
			Value: color.White,
		})
		k, v = params.MethodWithColor(`"%3s`)
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})

		info = append(info, types.Arg{
			Key:   params.Path,
			Value: color.HiWhite,
		})

		info = append(info, types.Arg{
			Key:   params.Protocol + `"`,
			Value: color.HiWhite,
		})
		k, v = params.StatusCodeWithColor("%3d")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})

		k, v = params.LatencyWithColor("%8s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		info = append(info, types.Arg{
			Key:   byteCountSI(int64(params.BodySize)),
			Value: color.White,
		})
		info = append(info, types.Arg{
			Key:   params.UserAgent,
			Value: color.Blue,
		})

		if params.ErrorMessage != "" {
			info = append(info, types.Arg{
				Key:   "\n",
				Value: color.Normal,
			})
			info = append(info, types.Arg{
				Key:   params.ErrorMessage,
				Value: color.Red,
			})
		}
		w.Logger.Info(info)
	}
}
