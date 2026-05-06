package text

import (
	"net/url"
	"regexp"
)

var urlRe = regexp.MustCompile(`https?://[^\s"'<>]+`)

func LeaveOnlyDomainInURLs(input string) string {
	return urlRe.ReplaceAllStringFunc(input, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}

		if u.Scheme == "" || u.Host == "" {
			return raw
		}

		return u.Scheme + "://" + u.Host
	})
}
