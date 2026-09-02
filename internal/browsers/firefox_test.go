package browsers

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestReadFirefoxCookies(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cookies.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE moz_cookies (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value TEXT,
			host TEXT,
			path TEXT,
			expiry INTEGER,
			isSecure INTEGER,
			isHttpOnly INTEGER,
			sameSite INTEGER
		);
		INSERT INTO moz_cookies (name, value, host, path, expiry, isSecure, isHttpOnly, sameSite)
		VALUES ('__Secure-next-auth.session-token', 'tok', '.chatgpt.com', '/', 1999999999, 1, 1, 0);
		INSERT INTO moz_cookies (name, value, host, path, expiry, isSecure, isHttpOnly, sameSite)
		VALUES ('other', 'nope', '.example.com', '/', 1999999999, 1, 0, 1);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cookies, err := extractFirefox(Source{
		Name:        "Firefox",
		Profile:     "default",
		CookiesPath: dbPath,
		Kind:        KindFirefox,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}

	filtered := FilterChatGPT(cookies)
	if len(filtered) != 1 || filtered[0].Value != "tok" {
		t.Fatalf("filtered = %+v", filtered)
	}
	if !HasChatGPTSession(filtered) {
		t.Fatal("expected session token")
	}
}
