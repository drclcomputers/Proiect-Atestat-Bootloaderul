package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const sessionCookieName = "session_token"
const sessionDuration = 7 * 24 * time.Hour

// hashPassword transforma parola într-un hash pentru a o nu salva direct în baza de date (din motive de securitate)
func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func checkPassword(password, salt, wantHash string) bool {
	got := hashPassword(password, salt)
	return subtle.ConstantTimeCompare([]byte(got), []byte(wantHash)) == 1
}

func getUserByUsername(username string) (*User, error) {
	u := &User{}
	var isAdmin int
	err := db.QueryRow(
		`SELECT id, username, password_hash, salt, is_admin, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Salt, &isAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}

func getUserByID(id int64) (*User, error) {
	u := &User{}
	var isAdmin int
	err := db.QueryRow(
		`SELECT id, username, password_hash, salt, is_admin, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Salt, &isAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}

func createSession(userID int64) (string, error) {
	token := randomHex(32)
	_, err := db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, time.Now().Add(sessionDuration),
	)
	return token, err
}

func destroySession(token string) {
	db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
}

// currentUser returnează userul conectat
func currentUser(r *http.Request) *User {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	var userID int64
	var expiresAt time.Time
	err = db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`,
		cookie.Value,
	).Scan(&userID, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		return nil
	}
	u, err := getUserByID(userID)
	if err != nil {
		return nil
	}
	return u
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionDuration),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || !u.IsAdmin {
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

var errUsernameTaken = errors.New("username already taken")

func registerUser(username, password string) (*User, error) {
	if len(username) < 3 || len(password) < 6 {
		return nil, errors.New("nume de utilizator (min 3 caractere) sau parola (min 6 caractere) invalide")
	}
	_, err := getUserByUsername(username)
	if err == nil {
		return nil, errUsernameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	salt := randomHex(16)
	hash := hashPassword(password, salt)
	res, err := db.Exec(
		`INSERT INTO users (username, password_hash, salt, is_admin) VALUES (?, ?, ?, 0)`,
		username, hash, salt,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return getUserByID(id)
}

// Handlere HTTP care servesc fișierele HTML serverului
func registerPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "register", PageData{Title: "Inregistrare"})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	u, err := registerUser(username, password)
	if err != nil {
		msg := "Nu am putut crea contul."
		if errors.Is(err, errUsernameTaken) {
			msg = "Acest nume de utilizator este deja folosit."
		}
		render(w, r, "register", PageData{Title: "Inregistrare", Flash: msg, FlashKind: "error"})
		return
	}
	token, err := createSession(u.ID)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "login", PageData{Title: "Autentificare"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	u, err := getUserByUsername(username)
	if err != nil || !checkPassword(password, u.Salt, u.PasswordHash) {
		render(w, r, "login", PageData{Title: "Autentificare", Flash: "Utilizator sau parola incorecte.", FlashKind: "error"})
		return
	}
	token, err := createSession(u.ID)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token)
	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		destroySession(cookie.Value)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
