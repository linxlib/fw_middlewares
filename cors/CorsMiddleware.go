package cors

import (
	"errors"
	"fmt"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var _ fw.IMiddlewareGlobal = (*CorsMiddleware)(nil)

type CorsMiddleware struct {
	*fw.MiddlewareGlobal
	config Config
}

func (c *CorsMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	cors := newCors(c.config)
	return func(context *fw.Context) {
		if cors.applyCors(context) {
			ctx.Next(context)
		}
	}
}

func NewDefaultCorsMiddleware() *CorsMiddleware {
	return &CorsMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal("CorsMiddleware"),
		config:           DefaultConfig(),
	}
}

func NewCorsMiddleware(config Config) *CorsMiddleware {
	return &CorsMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal("CorsMiddleware"),
		config:           config,
	}
}
func (c *CorsMiddleware) Router(ctx *fw.MiddlewareContext) []*fw.RouteItem {
	cors := newCors(c.config)
	return []*fw.RouteItem{&fw.RouteItem{
		Method: "OPTIONS",
		Path:   "/{all:*}",
		IsHide: false,
		H: func(context *fw.Context) {
			cors.applyCors(context)
		},
		Middleware: c,
	}}
}

type converter func(string) string

func generateNormalHeaders(c Config) http.Header {
	headers := make(http.Header)
	if c.AllowCredentials {
		headers.Set("Access-Control-Allow-Credentials", "true")
	}
	if len(c.ExposeHeaders) > 0 {
		exposeHeaders := convert(normalize(c.ExposeHeaders), http.CanonicalHeaderKey)
		headers.Set("Access-Control-Expose-Headers", strings.Join(exposeHeaders, ","))
	}
	if c.AllowAllOrigins {
		headers.Set("Access-Control-Allow-Origin", "*")
	} else {
		headers.Set("Vary", "Origin")
	}
	return headers
}
func generatePreflightHeaders(c Config) http.Header {
	headers := make(http.Header)
	if c.AllowCredentials {
		headers.Set("Access-Control-Allow-Credentials", "true")
	}
	if len(c.AllowMethods) > 0 {
		allowMethods := convert(normalize(c.AllowMethods), strings.ToUpper)
		value := strings.Join(allowMethods, ",")
		headers.Set("Access-Control-Allow-Methods", value)
	}
	if len(c.AllowHeaders) > 0 {
		allowHeaders := convert(normalize(c.AllowHeaders), http.CanonicalHeaderKey)
		value := strings.Join(allowHeaders, ",")
		headers.Set("Access-Control-Allow-Headers", value)
	}
	if c.MaxAge > time.Duration(0) {
		value := strconv.FormatInt(int64(c.MaxAge/time.Second), 10)
		headers.Set("Access-Control-Max-Age", value)
	}

	if c.AllowPrivateNetwork {
		headers.Set("Access-Control-Allow-Private-Network", "true")
	}

	if c.AllowAllOrigins {
		headers.Set("Access-Control-Allow-Origin", "*")
	} else {
		// Always set Vary headers
		// see https://github.com/rs/cors/issues/10,
		// https://github.com/rs/cors/commit/dbdca4d95feaa7511a46e6f1efb3b3aa505bc43f#commitcomment-12352001

		headers.Add("Vary", "Origin")
		headers.Add("Vary", "Access-Control-Request-Method")
		headers.Add("Vary", "Access-Control-Request-Headers")
	}
	return headers
}

func normalize(values []string) []string {
	if values == nil {
		return nil
	}
	distinctMap := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		value = strings.ToLower(value)
		if _, seen := distinctMap[value]; !seen {
			normalized = append(normalized, value)
			distinctMap[value] = true
		}
	}
	return normalized
}

func convert(s []string, c converter) []string {
	var out []string
	for _, i := range s {
		out = append(out, c(i))
	}
	return out
}

type cors struct {
	allowAllOrigins            bool
	allowCredentials           bool
	allowOriginFunc            func(string) bool
	allowOriginWithContextFunc func(*fw.Context, string) bool
	allowOrigins               []string
	normalHeaders              http.Header
	preflightHeaders           http.Header
	wildcardOrigins            [][]string
	optionsResponseStatusCode  int
}

var (
	DefaultSchemas = []string{
		"http://",
		"https://",
	}
	ExtensionSchemas = []string{
		"chrome-extension://",
		"safari-extension://",
		"moz-extension://",
		"ms-browser-extension://",
	}
	FileSchemas = []string{
		"file://",
	}
	WebSocketSchemas = []string{
		"ws://",
		"wss://",
	}
)

func newCors(config Config) *cors {
	if err := config.Validate(); err != nil {
		panic(err.Error())
	}

	for _, origin := range config.AllowOrigins {
		if origin == "*" {
			config.AllowAllOrigins = true
		}
	}

	if config.OptionsResponseStatusCode == 0 {
		config.OptionsResponseStatusCode = http.StatusNoContent
	}

	return &cors{
		allowOriginFunc:            config.AllowOriginFunc,
		allowOriginWithContextFunc: config.AllowOriginWithContextFunc,
		allowAllOrigins:            config.AllowAllOrigins,
		allowCredentials:           config.AllowCredentials,
		allowOrigins:               normalize(config.AllowOrigins),
		normalHeaders:              generateNormalHeaders(config),
		preflightHeaders:           generatePreflightHeaders(config),
		wildcardOrigins:            config.parseWildcardRules(),
		optionsResponseStatusCode:  config.OptionsResponseStatusCode,
	}
}

func (cors *cors) applyCors(c *fw.Context) (v bool) {
	origin := conv.String(c.GetFastContext().Request.Header.Peek("Origin"))
	if len(origin) == 0 {
		// request is not a CORS request
		v = true
		return
	}
	host := conv.String(c.GetFastContext().Host())

	if origin == "http://"+host || origin == "https://"+host {
		// request is not a CORS request but have origin header.
		// for example, use fetch api
		v = true
		return
	}

	if !cors.isOriginValid(c, origin) {
		c.GetFastContext().SetStatusCode(http.StatusForbidden)
		v = false
		return
	}

	if conv.String(c.GetFastContext().Method()) == "OPTIONS" {
		cors.handlePreflight(c)
		defer func() {
			c.GetFastContext().SetStatusCode(cors.optionsResponseStatusCode)
			v = false
		}()
	} else {
		cors.handleNormal(c)
		v = true
	}

	if !cors.allowAllOrigins {
		c.GetFastContext().Response.Header.Set("Access-Control-Allow-Origin", origin)
	}
	return
}

func (cors *cors) validateWildcardOrigin(origin string) bool {
	for _, w := range cors.wildcardOrigins {
		if w[0] == "*" && strings.HasSuffix(origin, w[1]) {
			return true
		}
		if w[1] == "*" && strings.HasPrefix(origin, w[0]) {
			return true
		}
		if strings.HasPrefix(origin, w[0]) && strings.HasSuffix(origin, w[1]) {
			return true
		}
	}

	return false
}

func (cors *cors) isOriginValid(c *fw.Context, origin string) bool {
	valid := cors.validateOrigin(origin)
	if !valid && cors.allowOriginWithContextFunc != nil {
		valid = cors.allowOriginWithContextFunc(c, origin)
	}
	return valid
}

func (cors *cors) validateOrigin(origin string) bool {
	if cors.allowAllOrigins {
		return true
	}
	for _, value := range cors.allowOrigins {
		if value == origin {
			return true
		}
	}
	if len(cors.wildcardOrigins) > 0 && cors.validateWildcardOrigin(origin) {
		return true
	}
	if cors.allowOriginFunc != nil {
		return cors.allowOriginFunc(origin)
	}
	return false
}

func (cors *cors) handlePreflight(c *fw.Context) {

	for key, value := range cors.preflightHeaders {
		c.GetFastContext().Response.Header.Set(key, strings.Join(value, ";"))
	}
}

func (cors *cors) handleNormal(c *fw.Context) {
	for key, value := range cors.normalHeaders {
		c.GetFastContext().Response.Header.Set(key, strings.Join(value, ";"))
	}
}

// Config represents all available options for the middleware.
type Config struct {
	AllowAllOrigins bool

	// AllowOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// Default value is []
	AllowOrigins []string

	// AllowOriginFunc is a custom function to validate the origin. It takes the origin
	// as an argument and returns true if allowed or false otherwise. If this option is
	// set, the content of AllowOrigins is ignored.
	AllowOriginFunc func(origin string) bool

	// Same as AllowOriginFunc except also receives the full request context.
	// This function should use the context as a read only source and not
	// have any side effects on the request, such as aborting or injecting
	// values on the request.
	AllowOriginWithContextFunc func(c *fw.Context, origin string) bool

	// AllowMethods is a list of methods the client is allowed to use with
	// cross-domain requests. Default value is simple methods (GET, POST, PUT, PATCH, DELETE, HEAD, and OPTIONS)
	AllowMethods []string

	// AllowPrivateNetwork indicates whether the response should include allow private network header
	AllowPrivateNetwork bool

	// AllowHeaders is list of non simple headers the client is allowed to use with
	// cross-domain requests.
	AllowHeaders []string

	// AllowCredentials indicates whether the request can include user credentials like
	// cookies, HTTP authentication or client side SSL certificates.
	AllowCredentials bool

	// ExposeHeaders indicates which headers are safe to expose to the API of a CORS
	// API specification
	ExposeHeaders []string

	// MaxAge indicates how long (with second-precision) the results of a preflight request
	// can be cached
	MaxAge time.Duration

	// Allows to add origins like http://some-domain/*, https://api.* or http://some.*.subdomain.com
	AllowWildcard bool

	// Allows usage of popular browser extensions schemas
	AllowBrowserExtensions bool

	// Allows to add custom schema like tauri://
	CustomSchemas []string

	// Allows usage of WebSocket protocol
	AllowWebSockets bool

	// Allows usage of file:// schema (dangerous!) use it only when you 100% sure it's needed
	AllowFiles bool

	// Allows to pass custom OPTIONS response status code for old browsers / clients
	OptionsResponseStatusCode int
}

// AddAllowMethods is allowed to add custom methods
func (c *Config) AddAllowMethods(methods ...string) {
	c.AllowMethods = append(c.AllowMethods, methods...)
}

// AddAllowHeaders is allowed to add custom headers
func (c *Config) AddAllowHeaders(headers ...string) {
	c.AllowHeaders = append(c.AllowHeaders, headers...)
}

// AddExposeHeaders is allowed to add custom expose headers
func (c *Config) AddExposeHeaders(headers ...string) {
	c.ExposeHeaders = append(c.ExposeHeaders, headers...)
}

func (c Config) getAllowedSchemas() []string {
	allowedSchemas := DefaultSchemas
	if c.AllowBrowserExtensions {
		allowedSchemas = append(allowedSchemas, ExtensionSchemas...)
	}
	if c.AllowWebSockets {
		allowedSchemas = append(allowedSchemas, WebSocketSchemas...)
	}
	if c.AllowFiles {
		allowedSchemas = append(allowedSchemas, FileSchemas...)
	}
	if c.CustomSchemas != nil {
		allowedSchemas = append(allowedSchemas, c.CustomSchemas...)
	}
	return allowedSchemas
}

func (c Config) validateAllowedSchemas(origin string) bool {
	allowedSchemas := c.getAllowedSchemas()
	for _, schema := range allowedSchemas {
		if strings.HasPrefix(origin, schema) {
			return true
		}
	}
	return false
}

// Validate is check configuration of user defined.
func (c Config) Validate() error {
	hasOriginFn := c.AllowOriginFunc != nil
	hasOriginFn = hasOriginFn || c.AllowOriginWithContextFunc != nil

	if c.AllowAllOrigins && (hasOriginFn || len(c.AllowOrigins) > 0) {
		originFields := strings.Join([]string{
			"AllowOriginFunc",
			"AllowOriginFuncWithContext",
			"AllowOrigins",
		}, " or ")
		return fmt.Errorf(
			"conflict settings: all origins enabled. %s is not needed",
			originFields,
		)
	}
	if !c.AllowAllOrigins && !hasOriginFn && len(c.AllowOrigins) == 0 {
		return errors.New("conflict settings: all origins disabled")
	}
	for _, origin := range c.AllowOrigins {
		if !strings.Contains(origin, "*") && !c.validateAllowedSchemas(origin) {
			return errors.New("bad origin: origins must contain '*' or include " + strings.Join(c.getAllowedSchemas(), ","))
		}
	}
	return nil
}

func (c Config) parseWildcardRules() [][]string {
	var wRules [][]string

	if !c.AllowWildcard {
		return wRules
	}

	for _, o := range c.AllowOrigins {
		if !strings.Contains(o, "*") {
			continue
		}

		if c := strings.Count(o, "*"); c > 1 {
			panic(errors.New("only one * is allowed").Error())
		}

		i := strings.Index(o, "*")
		if i == 0 {
			wRules = append(wRules, []string{"*", o[1:]})
			continue
		}
		if i == (len(o) - 1) {
			wRules = append(wRules, []string{o[:i], "*"})
			continue
		}

		wRules = append(wRules, []string{o[:i], o[i+1:]})
	}

	return wRules
}

// DefaultConfig returns a generic default configuration mapped to localhost.
func DefaultConfig() Config {
	return Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}
}
