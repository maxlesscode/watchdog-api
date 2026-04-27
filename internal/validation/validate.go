package validation

import (
	"net"
	"net/url"

	"github.com/maxlesscode/watchdog/internal/models"
	"github.com/maxlesscode/watchdog/internal/netutil"
)

func ValidateProduct(p models.Product) map[string]string {
	errs := make(map[string]string)

	if len(p.Name) == 0 {
		errs["name"] = "is required"
	}

	if p.TargetPrice <= 0 {
		errs["target_price"] = "must be greater than zero"
	}

	if len(p.URL) == 0 {
		errs["url"] = "is required"
	} else if u, err := url.Parse(p.URL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		errs["url"] = "must be a valid http or https URL"
	} else if ip := net.ParseIP(u.Hostname()); ip != nil && netutil.IsPrivateIP(ip) {
		errs["url"] = "URL must not target a private or internal address"
	}

	return errs
}
