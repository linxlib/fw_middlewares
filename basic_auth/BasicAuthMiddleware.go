package c

import (
	"crypto/subtle"
	"encoding/base64"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"net/http"
	"strconv"
)

var _ fw.IMiddlewareCtl = (*BasicAuthMiddleware)(nil)

const (
	AuthUserKey      = "user"
	AuthProxyUserKey = "proxy_user"
)

type Accounts map[string]string
type authPair struct {
	user  string
	value string
}
type authPairs []authPair

func (a authPairs) searchCredential(authValue string) (string, bool) {
	if authValue == "" {
		return "", false
	}
	for _, pair := range a {
		if subtle.ConstantTimeCompare(conv.Bytes(pair.value), conv.Bytes(authValue)) == 1 {
			return pair.user, true
		}
	}
	return "", false
}
func processAccounts(accounts Accounts) authPairs {
	length := len(accounts)
	if length <= 0 {
		panic("Empty list of authorized credentials")
	}
	pairs := make(authPairs, 0, length)
	for user, password := range accounts {
		if user == "" {
			panic("User can not be empty")
		}
		value := authorizationHeader(user, password)
		pairs = append(pairs, authPair{
			value: value,
			user:  user,
		})
	}
	return pairs
}
func authorizationHeader(user, password string) string {
	base := user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString(conv.Bytes(base))
}

type BasicAuthMiddleware struct {
	*fw.MiddlewareCtl
	realm string
	proxy bool
	pairs authPairs
}

func (b *BasicAuthMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	if v := ctx.GetParam("proxy"); v != "" {
		b.proxy = v == "true"
	}
	ctx.DelParam("proxy")
	if v := ctx.GetParam("realm"); v != "" {
		b.realm = v
	}
	if b.realm == "" {
		if b.proxy {
			b.realm = "Proxy Authorization Required"
		} else {
			b.realm = "Authorization Required"
		}
	}
	b.realm = "Basic realm=" + strconv.Quote(b.realm)
	ctx.DelParam("realm")
	accounts := make(Accounts)
	ctx.VisitParams(func(key string, value []string) {
		accounts[key] = value[0]
	})

	b.pairs = processAccounts(accounts)

	if b.proxy {
		return func(context *fw.Context) {
			bs := context.GetFastContext().Request.Header.Peek("Proxy-Authorization")
			proxyUser, found := b.pairs.searchCredential(conv.String(bs))
			if !found {
				// Credentials doesn't match, we return 407 and abort handlers chain.
				context.GetFastContext().Response.Header.Set("Proxy-Authenticate", b.realm)
				context.GetFastContext().Response.SetStatusCode(http.StatusProxyAuthRequired)
				return
			}
			context.Set(AuthProxyUserKey, proxyUser)
			ctx.Next(context)
		}
	} else { //basic auth
		return func(context *fw.Context) {
			bs := context.GetFastContext().Request.Header.Peek("Authorization")
			user, found := b.pairs.searchCredential(conv.String(bs))
			if !found {
				// Credentials doesn't match, we return 407 and abort handlers chain.
				context.GetFastContext().Response.Header.Set("WWW-Authenticate", b.realm)
				context.GetFastContext().Response.SetStatusCode(http.StatusUnauthorized)
				return
			}
			context.Set(AuthUserKey, user)
			ctx.Next(context)
		}
	}

}

func NewBasicAuthMiddleware() fw.IMiddlewareCtl {
	return &BasicAuthMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl("BasicAuth", "BasicAuth"),
	}
}
