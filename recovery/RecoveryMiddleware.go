package recovery

import (
	"bytes"
	"fmt"
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"strings"
	"time"
)

type RecoveryOptions struct {

	// if true, panic info and request info will show as web page
	// else output json to response
	// won't output stack trace to response unless the environment FW_DEBUG=true
	NiceWeb bool
	// whether output to console
	// won't output stack trace to console unless the environment FW_DEBUG=true
	Console bool
}

var _ fw.IMiddlewareGlobal = (*RecoveryMiddleware)(nil)

// RecoveryMiddleware globally recover from panic
type RecoveryMiddleware struct {
	*fw.MiddlewareGlobal
	options *RecoveryOptions
	Logger  *logrus.Logger `inject:""`
	isDebug bool
}

func (s *RecoveryMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		defer func() {
			if err := recover(); err != nil {
				var errMsg string
				switch err.(type) {
				case error:
					errMsg = err.(error).Error()
				case string:
					errMsg = err.(string)
				default:
					errMsg = ""
				}

				//var brokenPipe bool
				//if ne, ok := err.(*net.OpError); ok {
				//	if se, ok := ne.Err.(*os.SyscallError); ok {
				//		if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
				//			brokenPipe = true
				//		}
				//	}
				//}
				stack := stack(3, 12)
				if s.Logger != nil {
					//DUMP http request、headers etc.
					reqStr := &strings.Builder{}
					reqStr.WriteString(fmt.Sprintf("RemoteIP: %s\n", context.RemoteIP()))
					reqStr.WriteString(fmt.Sprintf("Host: %s\n", context.GetFastContext().Host()))
					reqStr.WriteString(fmt.Sprintf("Method: %s\n", context.Method()))
					reqStr.WriteString(fmt.Sprintf("URI: %s\n", context.GetFastContext().RequestURI()))
					reqStr.WriteString("Headers:\n")
					context.GetFastContext().Request.Header.VisitAll(func(k, v []byte) {
						reqStr.WriteString(fmt.Sprintf(" %s: %s\n", k, v))
					})
					reqStr.WriteString(fmt.Sprintf("Body: %s\n", context.GetFastContext().PostBody()))

					if s.isDebug {

						s.Logger.Printf(
							"["+color.HiCyan.Render("Recovery")+"] panic recovered: %s\n"+color.HiYellow.Render("Request:")+"\n%s"+color.HiYellow.Render("Stack Trace:")+"\n%s\n",
							color.HiRed.Render(errMsg),
							color.Blue.Render(reqStr.String()),
							color.HiMagenta.Render(conv.String(stack)))
					} else {
						s.Logger.Printf(
							"["+color.HiCyan.Render("Recovery")+"] panic recovered: %s\n%s",
							color.HiRed.Render(errMsg),
							color.Blue.Render(reqStr.String()))
					}

				}
				context.String(500, errMsg)

			}
		}()
		ctx.Next(context)
	}
}

const recoveryName = "Recovery"

func NewRecoveryMiddleware(o *RecoveryOptions, logger *logrus.Logger) fw.IMiddlewareGlobal {
	isDebug := os.Getenv("FW_DEBUG") == ""
	return &RecoveryMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(recoveryName),
		options:          o,
		isDebug:          isDebug,
		Logger:           logger,
	}
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

// stack returns a nicely formatted stack frame, skipping skip frames.
func stack(skip int, max int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; i <= max; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%X)\n", file, line, pc)
		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}
func timeFormat(t time.Time) string {
	timeString := t.Format("2006/01/02 - 15:04:05")
	return timeString
}
