package sites

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LegacyZones maps old zone names to their current IANA names.
//
// Browsers (ICU) still report and list several old names as canonical ("Asia/Calcutta"), but
// Postgres builds without the legacy tzdata (the official Debian-based image among them) only
// know the current ones ("Asia/Kolkata"). All time math runs in Postgres, so sites always store
// the current name.
var LegacyZones = map[string]string{
	"Africa/Asmera":         "Africa/Asmara",
	"America/Buenos_Aires":  "America/Argentina/Buenos_Aires",
	"America/Catamarca":     "America/Argentina/Catamarca",
	"America/Coral_Harbour": "America/Atikokan",
	"America/Cordoba":       "America/Argentina/Cordoba",
	"America/Godthab":       "America/Nuuk",
	"America/Indianapolis":  "America/Indiana/Indianapolis",
	"America/Jujuy":         "America/Argentina/Jujuy",
	"America/Louisville":    "America/Kentucky/Louisville",
	"America/Mendoza":       "America/Argentina/Mendoza",
	"Asia/Calcutta":         "Asia/Kolkata",
	"Asia/Dacca":            "Asia/Dhaka",
	"Asia/Katmandu":         "Asia/Kathmandu",
	"Asia/Rangoon":          "Asia/Yangon",
	"Asia/Saigon":           "Asia/Ho_Chi_Minh",
	"Asia/Thimbu":           "Asia/Thimphu",
	"Asia/Ulan_Bator":       "Asia/Ulaanbaatar",
	"Atlantic/Faeroe":       "Atlantic/Faroe",
	"Europe/Kiev":           "Europe/Kyiv",
	"Europe/Uzhgorod":       "Europe/Kyiv",
	"Europe/Zaporozhye":     "Europe/Kyiv",
	"Pacific/Enderbury":     "Pacific/Kanton",
	"Pacific/Ponape":        "Pacific/Pohnpei",
	"Pacific/Truk":          "Pacific/Chuuk",
}

// zoneAreas are the prefixes of the zones offered in the dashboard's picker.
const zoneAreas = `^(Africa|America|Antarctica|Asia|Atlantic|Australia|Europe|Indian|Pacific)/`

type zoneCache struct {
	once  sync.Once
	names []string
	err   error
}

// Timezones lists the zones both Postgres and Go understand, UTC first. It is read once.
func (s *Store) Timezones(ctx context.Context) ([]string, error) {
	s.zones.once.Do(func() {
		rows, err := s.pool.Query(ctx, `SELECT name FROM pg_timezone_names WHERE name ~ $1 ORDER BY name`, zoneAreas)
		if err != nil {
			s.zones.err = err
			return
		}
		defer rows.Close()
		names := []string{"UTC"}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				s.zones.err = err
				return
			}
			if _, legacy := LegacyZones[name]; legacy {
				continue // it would be saved under its current name, which is listed too
			}
			if _, err := time.LoadLocation(name); err == nil {
				names = append(names, name)
			}
		}
		s.zones.names, s.zones.err = names, rows.Err()
	})
	if s.zones.err != nil {
		// Let the next call try again.
		err := s.zones.err
		s.zones = zoneCache{}
		return nil, err
	}
	return s.zones.names, nil
}

// validTimezone returns the zone to store: legacy names become current ones, and the result must
// be known to both Go (period math) and Postgres (bucketing).
func (s *Store) validTimezone(ctx context.Context, tz string) (string, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return "UTC", nil
	}
	if current, ok := LegacyZones[tz]; ok {
		tz = current
	}
	invalid := &ValidationError{"timezone", "unknown time zone; use an IANA name such as Europe/Berlin"}
	if tz == "Local" {
		return "", invalid
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return "", invalid
	}
	var known bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)`, tz).Scan(&known); err != nil {
		return "", fmt.Errorf("checking the time zone: %w", err)
	}
	if !known {
		return "", invalid
	}
	return tz, nil
}
