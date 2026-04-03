package src

var HopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Transfer-Encoding",
	"Upgrade",
}

var HopByHopHeadersMap = func() map[string]bool {
	m := make(map[string]bool)
	for _, h := range HopByHopHeaders {
		m[h] = true
	}

	return m
}()

const (
	DefaultPort           = 1355
	XForwardedForHeader   = "X-Forwarded-For"
	XForwardedPortHeader  = "X-Forwarded-Port"
	XForwardedHostHeader  = "X-Forwarded-Host"
	XForwardedProtoHeader = "X-Forwarded-Proto"
	XPortlessHopsHeader   = "X-Portless-Hops"

	ReverseProxyImpl = "reverse"
	ManualProxyImpl  = "manual"

	MaxHops = 5
)
