package log

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/types"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type LogParams struct {
	// TimeStamp shows the time after the server returns a response.
	TimeStamp time.Time
	// StatusCode is HTTP response code.
	StatusCode int
	// Latency is how much time the server cost to process a certain request.
	Latency time.Duration
	// ClientIP equals Context's ClientIP method.
	ClientIP string
	// Method is the HTTP method given to the request.
	Method string
	// Path is a path the client requests.
	Path string
	// ErrorMessage is set if error has occurred in processing the request.
	ErrorMessage string
	// BodySize is the size of the Response Body
	BodySize int
	// Keys are the keys set on the request's context.
	Keys map[string]any

	Protocol  string
	UserAgent string
}

func (p *LogParams) TimeStampWithColor(f string) (string, color.Color) {
	return fmt.Sprintf(f, p.TimeStamp.Format(time.DateTime)), color.HiWhite
}
func (p *LogParams) LatencyWithColor(f string) (string, color.Color) {
	return fmt.Sprintf(f, p.Latency.String()), color.HiWhite
}
func (p *LogParams) ClientIPWithColor(f string) (string, color.Color) {
	return fmt.Sprintf(f, p.ClientIP), color.HiWhite
}

func (p *LogParams) StatusCodeWithColor(f string) (string, color.Color) {
	code := fmt.Sprintf(f, p.StatusCode)
	switch {
	case p.StatusCode >= http.StatusContinue && p.StatusCode < http.StatusOK:
		return code, color.White
	case p.StatusCode >= http.StatusOK && p.StatusCode < http.StatusMultipleChoices:
		return code, color.HiGreen
	case p.StatusCode >= http.StatusMultipleChoices && p.StatusCode < http.StatusBadRequest:
		return code, color.White
	case p.StatusCode >= http.StatusBadRequest && p.StatusCode < http.StatusInternalServerError:
		return code, color.Yellow
	default:
		return code, color.Red
	}
}

func (p *LogParams) MethodWithColor(f string) (string, color.Color) {
	m := fmt.Sprintf(f, p.Method)
	switch p.Method {
	case "GET":
		return m, color.Blue
	case "POST":
		return m, color.Cyan
	case "PUT":
		return m, color.Yellow
	case "DELETE":
		return m, color.Red
	case "PATCH":
		return m, color.Green
	case "HEAD":
		return m, color.Magenta
	case "OPTIONS":
		return m, color.White
	default:
		return m, color.Normal
	}
}

const (
	logAttr = "Log"
	logName = "Log"
)

func NewLogMiddleware(logger *logrus.Logger) fw.IMiddlewareCtl {
	return &LogMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl(logName, logAttr),
		Logger:        logger,
	}
}

var _ fw.IMiddlewareCtl = (*LogMiddleware)(nil)

// LogMiddleware
// for logging request info.
// can be used on Controller or Method
type LogMiddleware struct {
	*fw.MiddlewareCtl
	Logger *logrus.Logger `inject:""`
	// real_ip_header=CF-Connecting-IP
	realIPHeader string
}

func (w *LogMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	w.realIPHeader = ctx.GetParam("real_ip_header")
	return func(context *fw.Context) {
		fctx := context.GetFastContext()
		start := time.Now()
		params := &LogParams{}
		params.BodySize = len(fctx.PostBody())
		params.Path = conv.String(fctx.Request.RequestURI())
		// add Cloudflare CDN real ip header support
		if w.realIPHeader != "" {
			params.ClientIP = string(fctx.Request.Header.Peek(w.realIPHeader))
		}
		params.ClientIP = fctx.RemoteAddr().String()

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
		k, v := params.TimeStampWithColor("%20s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		k, v = params.ClientIPWithColor("%20s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		info = append(info, types.Arg{
			Key:   "-",
			Value: color.White,
		})
		k, v = params.MethodWithColor("%3s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		//k, v = params.Path

		info = append(info, types.Arg{
			Key:   params.Path,
			Value: color.White,
		})
		k, v = params.LatencyWithColor("%7s")
		info = append(info, types.Arg{
			Key:   k,
			Value: v,
		})
		info = append(info, types.Arg{
			Key:   byteCountSI(int64(params.BodySize)),
			Value: color.White,
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

// ByteCountSI 字节数转带单位
func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
