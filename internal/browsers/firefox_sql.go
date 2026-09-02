package browsers

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func readFirefoxCookies(dbPath string) ([]Cookie, error) {
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open firefox cookies: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name, value, host, path, expiry, isSecure, isHttpOnly, sameSite FROM moz_cookies`)
	if err != nil {
		rows, err = db.Query(`SELECT name, value, host, path, expiry, isSecure, isHttpOnly, 0 FROM moz_cookies`)
		if err != nil {
			return nil, fmt.Errorf("read firefox cookies: %w", err)
		}
	}
	defer rows.Close()

	var cookies []Cookie
	for rows.Next() {
		var (
			c        Cookie
			secure   int
			httpOnly int
			sameSite int
		)
		if err := rows.Scan(&c.Name, &c.Value, &c.Domain, &c.Path, &c.Expires, &secure, &httpOnly, &sameSite); err != nil {
			return nil, err
		}
		c.Secure = secure != 0
		c.HTTPOnly = httpOnly != 0
		c.SameSite = firefoxSameSite(sameSite)
		c.Session = c.Expires <= 0
		cookies = append(cookies, c)
	}
	return cookies, rows.Err()
}

func firefoxSameSite(v int) string {
	switch v {
	case 0:
		return "None"
	case 1:
		return "Lax"
	case 2:
		return "Strict"
	default:
		return ""
	}
}
