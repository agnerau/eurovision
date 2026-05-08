package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"eurovision/internal/db"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db           *sql.DB
	templates    map[string]*template.Template
	cookieSecret []byte
}

type Country struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Pick struct {
	CountryID int64 `json:"country_id"`
	Place     int   `json:"place"`
}

type LeaderRow struct {
	Username string
	Score    int
}

type PickRow struct {
	Username string
	Country  string
	Place    int
}

func main() {
	database := db.InitDB()

	secret := []byte(os.Getenv("SESSION_SECRET"))
	if len(secret) < 32 {
		log.Println("WARNING: SESSION_SECRET missing or short; generating temporary secret. Sessions will reset on restart.")
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatal(err)
		}
	}

	app := &App{
		db:           database,
		templates:    loadTemplates(),
		cookieSecret: secret,
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", app.index)
	mux.HandleFunc("/register", app.register)
	mux.HandleFunc("/login", app.login)
	mux.HandleFunc("/logout", app.logout)

	mux.HandleFunc("/home", app.requireAuth(app.home))
	mux.HandleFunc("/prediction/new", app.requireAuth(app.predictionNew))
	mux.HandleFunc("/prediction/edit", app.requireAuth(app.predictionEdit))
	mux.HandleFunc("/predictions/", app.userPredictions)

	mux.HandleFunc("/api/countries", app.apiCountries)
	mux.HandleFunc("/api/my-stats", app.requireAuth(app.apiMyStats))

	mux.HandleFunc("/admin/countries", app.adminCountries)
	mux.HandleFunc("/admin/lock", app.adminLock)
	mux.HandleFunc("/admin/winner", app.adminWinner)

	port := getenv("PORT", "8080")
	addr := "127.0.0.1:" + port

	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func loadTemplates() map[string]*template.Template {
	files := []string{
		"login.html",
		"register.html",
		"home.html",
		"prediction.html",
		"user_predictions.html",
	}

	tpls := make(map[string]*template.Template)

	for _, f := range files {
		tpls[f] = template.Must(template.ParseFiles("templates/" + f))
	}

	return tpls
}

func (a *App) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tpl, ok := a.templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}

	if err := tpl.Execute(w, data); err != nil {
		log.Println("template error:", err)
	}
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUserID(r); ok {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.render(w, "register.html", nil)

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if len(username) < 3 || len(password) < 1 {
			a.render(w, "register.html", map[string]any{
				"Error": "Username must be at least 3 characters and password at least 1 characters.",
			})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "password error", http.StatusInternalServerError)
			return
		}

		var id int64
		err = a.db.QueryRow(
			`INSERT INTO users(username, password_hash) VALUES($1, $2) RETURNING id`,
			username,
			string(hash),
		).Scan(&id)

		if err != nil {
			println(err.Error())
			a.render(w, "register.html", map[string]any{
				"Error": "Username already exists.",
			})
			return
		}

		a.setSession(w, id)
		http.Redirect(w, r, "/home", http.StatusSeeOther)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.render(w, "login.html", nil)

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		var id int64
		var hash string

		err := a.db.QueryRow(
			`SELECT id, password_hash FROM users WHERE username = $1`,
			username,
		).Scan(&id, &hash)

		if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			a.render(w, "login.html", map[string]any{
				"Error": "Invalid username or password.",
			})
			return
		}

		a.setSession(w, id)
		http.Redirect(w, r, "/home", http.StatusSeeOther)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.currentUserID(r)

	var hasPrediction bool

	err := a.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM stats
			WHERE user_id = $1
		)
	`, userID).Scan(&hasPrediction)

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	var locked bool

	err = a.db.QueryRow(`
		SELECT predictions_locked
		FROM app_settings
		WHERE id = TRUE
	`).Scan(&locked)

	if err != nil {
		http.Error(w, "settings error", http.StatusInternalServerError)
		return
	}

	var username string
	_ = a.db.QueryRow(`SELECT username FROM users WHERE id = $1`, userID).Scan(&username)

	leaderRows, err := a.db.Query(`
		SELECT 
			u.username,
			COALESCE(COUNT(w.id), 0) AS score
		FROM users u
		LEFT JOIN stats s ON s.user_id = u.id
		LEFT JOIN countries c ON c.id = s.country_id
		LEFT JOIN winner_countries w 
			ON w.country_id = c.id
			AND w.place = s.place
		GROUP BY u.id, u.username
		ORDER BY score DESC, u.username ASC
	`)
	if err != nil {
		println(err.Error())
		http.Error(w, "leaderboard error", http.StatusInternalServerError)
		return
	}
	defer leaderRows.Close()

	type LeaderRow struct {
		Index    int
		Username string
		Score    int
	}

	var leaders []LeaderRow
	i := 1

	for leaderRows.Next() {
		var row LeaderRow

		if err = leaderRows.Scan(&row.Username, &row.Score); err == nil {
			row.Index = i
			i++
			leaders = append(leaders, row)
		}
	}

	type PredictionUser struct {
		Username string
	}

	rows, err := a.db.Query(`
		SELECT DISTINCT u.username
		FROM stats s
		JOIN users u ON u.id = s.user_id
		WHERE u.id <> $1
		ORDER BY u.username ASC
	`, userID)

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	var predictionUsers []PredictionUser

	for rows.Next() {
		var u PredictionUser

		if err := rows.Scan(&u.Username); err == nil {
			predictionUsers = append(predictionUsers, u)
		}
	}

	a.render(w, "home.html", map[string]any{
		"Username":          username,
		"Leaders":           leaders,
		"PredictionUsers":   predictionUsers,
		"HasPrediction":     hasPrediction,
		"PredictionsLocked": locked,
	})
}

func (a *App) userPredictions(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/predictions/")

	rows, err := a.db.Query(`
		SELECT s.place, c.name
		FROM stats s
		JOIN users u ON u.id = s.user_id
		JOIN countries c ON c.id = s.country_id
		WHERE u.username = $1
		ORDER BY s.place ASC
	`, username)

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var picks []PickRow

	for rows.Next() {
		var p PickRow

		rows.Scan(&p.Place, &p.Country)

		picks = append(picks, p)
	}

	a.render(w, "user_predictions.html", map[string]any{
		"Username": username,
		"Picks":    picks,
		"Title":    username + "'s predictions",
	})
}

func (a *App) predictionNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, "prediction.html", map[string]any{
		"Edit":  false,
		"Title": "Create prediction",
	})
}

func (a *App) predictionEdit(w http.ResponseWriter, r *http.Request) {
	a.render(w, "prediction.html", map[string]any{
		"Edit":  true,
		"Title": "Edit prediction",
	})
}

func (a *App) apiCountries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := a.db.Query(`SELECT id, name FROM countries ORDER BY name ASC`)
	if err != nil {
		http.Error(w, "countries error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var countries []Country
	for rows.Next() {
		var c Country
		if err := rows.Scan(&c.ID, &c.Name); err == nil {
			countries = append(countries, c)
		}
	}

	writeJSON(w, countries)
}

func (a *App) apiMyStats(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.currentUserID(r)

	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(
			`SELECT country_id, place FROM stats WHERE user_id = $1 ORDER BY place ASC`,
			userID,
		)
		if err != nil {
			http.Error(w, "stats error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var picks []Pick
		for rows.Next() {
			var p Pick
			if err := rows.Scan(&p.CountryID, &p.Place); err == nil {
				picks = append(picks, p)
			}
		}

		writeJSON(w, map[string]any{"picks": picks})

	case http.MethodPost:
		var locked bool

		err := a.db.QueryRow(`
			SELECT predictions_locked
			FROM app_settings
			WHERE id = TRUE
		`).Scan(&locked)

		if err != nil {
			http.Error(w, "settings error", http.StatusInternalServerError)
			return
		}
		if locked {
			http.Error(w, "predictions locked", http.StatusForbidden)
			return
		}

		var payload struct {
			Picks []Pick `json:"picks"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if err := validatePicks(payload.Picks); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, "transaction error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`DELETE FROM stats WHERE user_id = $1`, userID); err != nil {
			http.Error(w, "delete error", http.StatusInternalServerError)
			return
		}

		stmt, err := tx.Prepare(`
			INSERT INTO stats(country_id, place, user_id)
			VALUES($1, $2, $3)
		`)
		if err != nil {
			http.Error(w, "prepare error", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		for _, p := range payload.Picks {
			if _, err := stmt.Exec(p.CountryID, p.Place, userID); err != nil {
				http.Error(w, "insert error: check country exists and places are unique", http.StatusBadRequest)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "commit error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func validatePicks(picks []Pick) error {
	seenCountries := map[int64]bool{}
	seenPlaces := map[int]bool{}

	for _, p := range picks {
		if p.CountryID <= 0 {
			return errors.New("invalid country id")
		}
		if p.Place <= 0 {
			return errors.New("invalid place")
		}
		if seenCountries[p.CountryID] {
			return errors.New("duplicate country")
		}
		if seenPlaces[p.Place] {
			return errors.New("duplicate place")
		}

		seenCountries[p.CountryID] = true
		seenPlaces[p.Place] = true
	}

	return nil
}

func (a *App) adminCountries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var countries []string

	if err := json.Unmarshal(body, &countries); err != nil {
		var payload struct {
			Countries []string `json:"countries"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "send JSON array or {countries:[...]}", http.StatusBadRequest)
			return
		}

		countries = payload.Countries
	}

	tx, err := a.db.Begin()
	if err != nil {
		http.Error(w, "transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO countries(name) VALUES($1) ON CONFLICT(name) DO NOTHING`)
	if err != nil {
		http.Error(w, "prepare error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	inserted := 0
	for _, name := range countries {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		res, err := stmt.Exec(name)
		if err != nil {
			http.Error(w, "insert error", http.StatusInternalServerError)
			return
		}

		n, _ := res.RowsAffected()
		inserted += int(n)
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "commit error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"ok":       true,
		"inserted": inserted,
	})
}

func (a *App) adminLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Lock bool `json:"lock"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	_, err := a.db.Exec(`
		UPDATE app_settings
		SET predictions_locked = $1
		WHERE id = TRUE
	`, payload.Lock)

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"locked": payload.Lock,
	})
}

func (a *App) adminWinner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		CountryID int64 `json:"country_id"`
		Place     int   `json:"place"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.CountryID <= 0 || payload.Place <= 0 {
		http.Error(w, "country_id and positive place required", http.StatusBadRequest)
		return
	}

	_, err := a.db.Exec(`
		INSERT INTO winner_countries(country_id, place)
		VALUES($1, $2)
		ON CONFLICT (place)
		DO UPDATE SET country_id = EXCLUDED.country_id
	`, payload.CountryID, payload.Place)

	if err != nil {
		http.Error(w, "winner insert/update error", http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.currentUserID(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

func (a *App) setSession(w http.ResponseWriter, userID int64) {
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	payload := fmt.Sprintf("%d:%d", userID, exp)
	sig := a.sign(payload)

	value := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    value,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) currentUserID(r *http.Request) (int64, bool) {
	c, err := r.Cookie("session")
	if err != nil {
		return 0, false
	}

	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return 0, false
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}

	payload := string(rawPayload)
	expectedSig := a.sign(payload)

	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return 0, false
	}

	pieces := strings.Split(payload, ":")
	if len(pieces) != 2 {
		return 0, false
	}

	userID, err := strconv.ParseInt(pieces[0], 10, 64)
	if err != nil {
		return 0, false
	}

	exp, err := strconv.ParseInt(pieces[1], 10, 64)
	if err != nil {
		return 0, false
	}

	if time.Now().Unix() > exp {
		return 0, false
	}

	return userID, true
}

func (a *App) sign(payload string) string {
	mac := hmac.New(sha256.New, a.cookieSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
