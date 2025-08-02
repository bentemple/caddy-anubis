package caddyanubis

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/bentemple/anubis"
	libanubis "github.com/bentemple/anubis/lib"
	"github.com/bentemple/anubis/lib/policy"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(AnubisMiddleware{})
	httpcaddyfile.RegisterHandlerDirective("anubis", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("anubis", httpcaddyfile.After, "templates")
}

type AnubisMiddleware struct {
	Target        *string `json:"target,omitempty"`
	AnubisPolicy  *policy.ParsedConfig
	AnubisServers map[string]*libanubis.Server
	Next          caddyhttp.Handler

	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (AnubisMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.anubis",
		New: func() caddy.Module { return new(AnubisMiddleware) },
	}
}

// validOptionalPort reports whether port is either an empty string
// or matches /^:\d*$/
func validOptionalPort(port string) bool {
	if port == "" {
		return true
	}
	if port[0] != ':' {
		return false
	}
	for _, b := range port[1:] {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func splitHostPort(hostPort string) (host, port string) {
	host = hostPort

	colon := strings.LastIndexByte(host, ':')
	if colon != -1 && validOptionalPort(host[colon:]) {
		host, port = host[:colon], host[colon+1:]
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}

	return
}

func splitSubDomainDomain(host string) (subdomain, domain string) {
	domain = host

	tldPeriod := strings.LastIndexByte(host, '.')
	if tldPeriod != -1 {
		withoutTLD := host[:tldPeriod]
		subdomainPeriod := strings.LastIndexByte(withoutTLD, '.')
		if subdomainPeriod != -1 {
			subdomain, domain = host[:subdomainPeriod], host[subdomainPeriod+1:]
		}
	}
	return
}

func DomainFromRequest(r *http.Request) string {
	host, _ := splitHostPort(r.Host)
	_, domain := splitSubDomainDomain(host)
	return domain
}

func (m *AnubisMiddleware) CreateAnubisServer(r *http.Request) error {
	domain := DomainFromRequest(r)

	if m.AnubisServers == nil {
		m.AnubisServers = make(map[string]*libanubis.Server)
	}

	if server := m.AnubisServers[domain]; server != nil {
		m.logger.Info("Anubis server already created for: " + domain)
		return nil
	}

	m.logger.Info("Creating new anubis server for domain: " + domain)

	anubisServer, err := libanubis.New(libanubis.Options{
		Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.logger.Info("Anubis middleware calling next" + r.URL.Path)

			if m.Target != nil {
				m.logger.Info("Target not null, redirecting")
				http.Redirect(w, r, *m.Target, http.StatusTemporaryRedirect)
			} else {
				err := m.Next.ServeHTTP(w, r)
				if err != nil {
					m.logger.Error("Anubis error when calling Next" + err.Error())
				}
			}
		}),
		CookieDomain:        domain,
		CookieDynamicDomain: true,
		CookieSecure:        true,
		Policy:              m.AnubisPolicy,
		ServeRobotsTXT:      true,
	})
	if err != nil {
		return err
	}

	m.AnubisServers[domain] = anubisServer

	return nil
}

// Provision implements caddy.Provisioner.
func (m *AnubisMiddleware) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger().Named("anubis")
	m.logger.Info("Anubis middleware provisioning")

	loadedPolicy, err := libanubis.LoadPoliciesOrDefault(ctx, "", anubis.DefaultDifficulty)
	if err != nil {
		return err
	}

	m.AnubisPolicy = loadedPolicy

	m.logger.Info("Anubis middleware provisioned")
	return nil
}

// Validate implements caddy.Validator.
func (m *AnubisMiddleware) Validate() error {
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *AnubisMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	domain := DomainFromRequest(r)
	err := m.CreateAnubisServer(r)
	if err != nil {
		m.logger.Error("Anubis middleware create server error" + err.Error())
		return err
	}
	m.logger.Info("Anubis middleware processing request, path: " + r.URL.Path)
	slog.SetLogLoggerLevel(slog.LevelDebug)
	m.logger.Info("Anubis middleware sending request")
	m.Next = next
	m.AnubisServers[domain].ServeHTTP(w, r)
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *AnubisMiddleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	// require an argument
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "target":
			if d.NextArg() {
				val := d.Val()
				m.Target = &val
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m AnubisMiddleware
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return &m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*AnubisMiddleware)(nil)
	_ caddy.Validator             = (*AnubisMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*AnubisMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*AnubisMiddleware)(nil)
)
