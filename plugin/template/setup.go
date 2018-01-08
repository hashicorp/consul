package template

import (
	"regexp"
	gotmpl "text/template"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

func init() {
	caddy.RegisterPlugin("template", caddy.Plugin{
		ServerType: "dns",
		Action:     setupTemplate,
	})
}

func setupTemplate(c *caddy.Controller) error {
	templates, err := templateParse(c)
	if err != nil {
		return plugin.Error("template", err)
	}

	c.OnStartup(OnStartupMetrics)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Handler{Next: next, Templates: templates}
	})

	return nil
}

func templateParse(c *caddy.Controller) (templates []template, err error) {
	templates = make([]template, 0)

	for c.Next() {
		t := template{}
		if !c.NextArg() {
			return nil, c.ArgErr()
		}

		class, ok := dns.StringToClass[c.Val()]
		if !ok {
			return nil, c.Errf("invalid query class %s", c.Val())
		}
		t.class = class

		if !c.NextArg() {
			return nil, c.ArgErr()
		}
		queryType, ok := dns.StringToType[c.Val()]
		if !ok {
			return nil, c.Errf("invalid RR type %s", c.Val())
		}
		t.qtype = queryType

		t.regex = make([]*regexp.Regexp, 0)
		templatePrefix := ""

		for _, regex := range c.RemainingArgs() {
			r, err := regexp.Compile(regex)
			if err != nil {
				return nil, c.Errf("could not parse regex: %s, %v", regex, err)
			}
			templatePrefix = templatePrefix + regex + " "
			t.regex = append(t.regex, r)
		}

		if len(t.regex) == 0 {
			t.regex = append(t.regex, regexp.MustCompile(".*"))
			templatePrefix = ".* "
		}

		t.answer = make([]*gotmpl.Template, 0)

		for c.NextBlock() {
			switch c.Val() {
			case "answer":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				for _, answer := range args {
					tmpl, err := gotmpl.New("answer").Parse(answer)
					if err != nil {
						return nil, c.Errf("could not compile template: %s, %v", c.Val(), err)
					}
					t.answer = append(t.answer, tmpl)
				}

			case "additional":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				for _, additional := range args {
					tmpl, err := gotmpl.New("additional").Parse(additional)
					if err != nil {
						return nil, c.Errf("could not compile template: %s, %v\n", c.Val(), err)
					}
					t.additional = append(t.additional, tmpl)
				}

			case "authority":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				for _, authority := range args {
					tmpl, err := gotmpl.New("authority").Parse(authority)
					if err != nil {
						return nil, c.Errf("could not compile template: %s, %v\n", c.Val(), err)
					}
					t.authority = append(t.authority, tmpl)
				}

			case "rcode":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				rcode, ok := dns.StringToRcode[c.Val()]
				if !ok {
					return nil, c.Errf("unknown rcode %s", c.Val())
				}
				t.rcode = rcode

			default:
				return nil, c.ArgErr()
			}
		}

		if len(t.answer) == 0 && len(t.additional) == 0 && t.rcode == dns.RcodeSuccess {
			return nil, c.Errf("no answer section for template %s %sfound", t.qtype, templatePrefix)
		}

		templates = append(templates, t)
	}

	return templates, nil
}
