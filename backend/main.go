package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Company struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Company  Company `json:"company"`
}

type Event struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Address          string    `json:"address"`
	StartsAt         time.Time `json:"startsAt"`
	EndsAt           time.Time `json:"endsAt"`
	RecurrenceMonths int       `json:"recurrenceMonths"`
	Description      string    `json:"description"`
	CreatedBy        string    `json:"createdBy"`
	AssignedTo       *string   `json:"assignedTo,omitempty"`
}

type AddressSummary struct {
	Address       string `json:"address"`
	NextEvent     *Event `json:"nextEvent,omitempty"`
	PreviousEvent *Event `json:"previousEvent,omitempty"`
}

type sessionClaims struct {
	UserID    string `json:"sub"`
	CompanyID string `json:"companyId"`
	ExpiresAt int64  `json:"exp"`
}

type Server struct {
	db            *pgxpool.Pool
	gatewayToken  string
	sessionSecret []byte
	origins       map[string]bool
}

//go:embed migrations/*.sql
var migrations embed.FS

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func id() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func newServer(ctx context.Context, databaseURL, gatewayToken, sessionSecret string) (*Server, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = "scheduler,public"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	server := &Server{db: pool, gatewayToken: gatewayToken, sessionSecret: []byte(sessionSecret), origins: map[string]bool{}}
	for _, origin := range strings.Split(env("FRONTEND_ORIGIN", "http://localhost:5173,http://localhost:8787"), ",") {
		server.origins[strings.TrimSpace(origin)] = true
	}
	if err := server.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := server.seedAdmin(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return server, nil
}

func (s *Server) migrate(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS scheduler`); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS scheduler.schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		var applied bool
		err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM scheduler.schema_migrations WHERE version = $1)`, file.Name()).Scan(&applied)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		sql, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO scheduler.schema_migrations (version) VALUES ($1)`, file.Name())
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) seedAdmin(ctx context.Context) error {
	username, password := strings.TrimSpace(os.Getenv("SEED_ADMIN_USERNAME")), os.Getenv("SEED_ADMIN_PASSWORD")
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("SEED_ADMIN_USERNAME and SEED_ADMIN_PASSWORD must be set together")
	}
	companyName := strings.TrimSpace(env("SEED_COMPANY_NAME", "Buldez Control"))
	var companyID string
	err := s.db.QueryRow(ctx, `INSERT INTO companies (id, name) VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, id(), companyName).Scan(&companyID)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO users (id, company_id, username, name, password_hash) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (username) DO NOTHING`, id(), companyID, username, env("SEED_ADMIN_NAME", username), string(hash))
	return err
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		fail(w, http.StatusBadRequest, "Invalid request data.")
		return false
	}
	return true
}

func (s *Server) signSession(claims sessionClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Server) session(r *http.Request) (sessionClaims, error) {
	var claims sessionClaims
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || parts[0] != "Bearer" {
		return claims, errors.New("missing bearer token")
	}
	token := strings.Split(parts[1], ".")
	if len(token) != 2 {
		return claims, errors.New("invalid token")
	}
	provided, err := base64.RawURLEncoding.DecodeString(token[1])
	if err != nil {
		return claims, err
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write([]byte(token[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return claims, errors.New("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(token[0])
	if err != nil || json.Unmarshal(payload, &claims) != nil || claims.UserID == "" || claims.CompanyID == "" || claims.ExpiresAt <= time.Now().Unix() {
		return sessionClaims{}, errors.New("expired token")
	}
	return claims, nil
}

func (s *Server) user(ctx context.Context, claims sessionClaims) (User, error) {
	var user User
	err := s.db.QueryRow(ctx, `SELECT u.id, u.username, u.name, c.id, c.name FROM users u JOIN companies c ON c.id = u.company_id WHERE u.id = $1 AND u.company_id = $2`, claims.UserID, claims.CompanyID).Scan(&user.ID, &user.Username, &user.Name, &user.Company.ID, &user.Company.Name)
	return user, err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !s.origins[origin] {
			fail(w, http.StatusForbidden, "Origin is not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Gateway-Token")), []byte(s.gatewayToken)) != 1 {
		fail(w, http.StatusUnauthorized, "Unauthorized gateway")
		return
	}
	if r.URL.Path == "/api/health" && r.Method == http.MethodGet {
		if err := s.db.Ping(r.Context()); err != nil {
			fail(w, http.StatusServiceUnavailable, "Database is unavailable")
			return
		}
		write(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path == "/api/auth/login" && r.Method == http.MethodPost {
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/auth/register" && r.Method == http.MethodPost {
		s.register(w, r)
		return
	}
	claims, err := s.session(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "Authentication is required")
		return
	}
	user, err := s.user(r.Context(), claims)
	if err != nil {
		fail(w, http.StatusUnauthorized, "Authentication is required")
		return
	}
	switch {
	case r.URL.Path == "/api/auth/me" && r.Method == http.MethodGet:
		write(w, http.StatusOK, user)
	case r.URL.Path == "/api/users" && r.Method == http.MethodGet:
		s.users(w, r, user.Company.ID)
	case r.URL.Path == "/api/events" && r.Method == http.MethodGet:
		s.events(w, r, user.Company.ID)
	case r.URL.Path == "/api/events" && r.Method == http.MethodPost:
		s.createEvent(w, r, user)
	case r.URL.Path == "/api/addresses/detail" && r.Method == http.MethodGet:
		s.addressHistory(w, r, user.Company.ID)
	case r.URL.Path == "/api/addresses" && r.Method == http.MethodGet:
		s.addresses(w, r, user.Company.ID)
	default:
		fail(w, http.StatusNotFound, "Request was not found")
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	var user User
	var hash string
	err := s.db.QueryRow(r.Context(), `SELECT u.id, u.username, u.name, u.password_hash, c.id, c.name FROM users u JOIN companies c ON c.id = u.company_id WHERE u.username = $1`, input.Username).Scan(&user.ID, &user.Username, &user.Name, &hash, &user.Company.ID, &user.Company.Name)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		fail(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}
	token, err := s.signSession(sessionClaims{UserID: user.ID, CompanyID: user.Company.ID, ExpiresAt: time.Now().Add(12 * time.Hour).Unix()})
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not start session")
		return
	}
	write(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CompanyName string `json:"companyName"`
		Name        string `json:"name"`
		Username    string `json:"username"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.CompanyName = strings.TrimSpace(input.CompanyName)
	input.Name = strings.TrimSpace(input.Name)
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if len([]rune(input.CompanyName)) < 2 || len([]rune(input.CompanyName)) > 200 || len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 100 || input.Username == "" || input.Password == "" || len(input.Password) > 72 {
		fail(w, http.StatusBadRequest, "Provide a company, your name, a username, and a password of up to 72 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create account")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create account")
		return
	}
	defer tx.Rollback(r.Context())
	company := Company{ID: id(), Name: input.CompanyName}
	result, err := tx.Exec(r.Context(), `INSERT INTO companies (id, name) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, company.ID, company.Name)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create company")
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, http.StatusConflict, "A company with this name already exists. Ask its administrator for an account")
		return
	}
	user := User{ID: id(), Username: input.Username, Name: input.Name, Company: company}
	result, err = tx.Exec(r.Context(), `INSERT INTO users (id, company_id, username, name, password_hash) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (username) DO NOTHING`, user.ID, company.ID, user.Username, user.Name, string(hash))
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create account")
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, http.StatusConflict, "This username is already in use")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "Could not create account")
		return
	}
	token, err := s.signSession(sessionClaims{UserID: user.ID, CompanyID: company.ID, ExpiresAt: time.Now().Add(12 * time.Hour).Unix()})
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not start session")
		return
	}
	write(w, http.StatusCreated, map[string]any{"token": token, "user": user})
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, companyID string) {
	rows, err := s.db.Query(r.Context(), `SELECT id, username, name FROM users WHERE company_id = $1 ORDER BY name, username`, companyID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load users")
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Name); err != nil {
			fail(w, http.StatusInternalServerError, "Could not load users")
			return
		}
		users = append(users, user)
	}
	write(w, http.StatusOK, users)
}

func dateRange(r *http.Request) (time.Time, time.Time, error) {
	from, err := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if err != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("invalid range")
	}
	return from, to.AddDate(0, 0, 1).Add(-time.Nanosecond), nil
}

func addMonthsClamped(at time.Time, months int) time.Time {
	first := time.Date(at.Year(), at.Month()+time.Month(months), 1, at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), at.Location())
	lastDay := first.AddDate(0, 1, -1).Day()
	day := at.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(first.Year(), first.Month(), day, at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), at.Location())
}

func occurrenceStarts(event Event, from, to time.Time) []Event {
	if event.RecurrenceMonths == 0 {
		if !event.StartsAt.Before(from) && !event.StartsAt.After(to) {
			return []Event{event}
		}
		return nil
	}
	months := (from.Year()-event.StartsAt.Year())*12 + int(from.Month()-event.StartsAt.Month())
	n := months / event.RecurrenceMonths
	if n < 0 {
		n = 0
	}
	result := []Event{}
	duration := event.EndsAt.Sub(event.StartsAt)
	for ; ; n++ {
		start := addMonthsClamped(event.StartsAt, n*event.RecurrenceMonths)
		if start.After(to) {
			break
		}
		if start.Before(from) {
			continue
		}
		occurrence := event
		occurrence.StartsAt, occurrence.EndsAt = start, start.Add(duration)
		result = append(result, occurrence)
	}
	return result
}

func scanEvents(rows pgx.Rows) ([]Event, error) {
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Title, &event.Address, &event.StartsAt, &event.EndsAt, &event.RecurrenceMonths, &event.Description, &event.CreatedBy, &event.AssignedTo); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

const eventQuery = `SELECT e.id, e.title, e.address, e.starts_at, e.ends_at, e.recurrence_months, e.description, creator.name, assignee.name
	FROM events e JOIN users creator ON creator.id = e.created_by LEFT JOIN users assignee ON assignee.id = e.assigned_to WHERE e.company_id = $1`

func (s *Server) events(w http.ResponseWriter, r *http.Request, companyID string) {
	from, to, err := dateRange(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "Choose a valid period of up to 366 days")
		return
	}
	rows, err := s.db.Query(r.Context(), eventQuery, companyID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load events")
		return
	}
	definitions, err := scanEvents(rows)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load events")
		return
	}
	occurrences := []Event{}
	for _, event := range definitions {
		occurrences = append(occurrences, occurrenceStarts(event, from, to)...)
	}
	sort.Slice(occurrences, func(i, j int) bool { return occurrences[i].StartsAt.Before(occurrences[j].StartsAt) })
	write(w, http.StatusOK, occurrences)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request, user User) {
	var input struct {
		Title            string    `json:"title"`
		Address          string    `json:"address"`
		StartsAt         time.Time `json:"startsAt"`
		EndsAt           time.Time `json:"endsAt"`
		RecurrenceMonths int       `json:"recurrenceMonths"`
		Description      string    `json:"description"`
		AssignedTo       *string   `json:"assignedTo"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Title, input.Address, input.Description = strings.TrimSpace(input.Title), strings.TrimSpace(input.Address), strings.TrimSpace(input.Description)
	if input.Title == "" || len([]rune(input.Title)) > 200 || input.Address == "" || len([]rune(input.Address)) > 500 || input.StartsAt.IsZero() || input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) || input.RecurrenceMonths < 0 || input.RecurrenceMonths > 120 || len([]rune(input.Description)) > 5000 {
		fail(w, http.StatusBadRequest, "Provide a title, address, valid start and end time, and a recurrence up to 120 months")
		return
	}
	if input.AssignedTo != nil && *input.AssignedTo != "" {
		var exists bool
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND company_id = $2)`, *input.AssignedTo, user.Company.ID).Scan(&exists); err != nil || !exists {
			fail(w, http.StatusBadRequest, "The assigned user is not in your company")
			return
		}
	} else {
		input.AssignedTo = nil
	}
	event := Event{ID: id(), Title: input.Title, Address: input.Address, StartsAt: input.StartsAt.UTC(), EndsAt: input.EndsAt.UTC(), RecurrenceMonths: input.RecurrenceMonths, Description: input.Description, CreatedBy: user.Name, AssignedTo: input.AssignedTo}
	_, err := s.db.Exec(r.Context(), `INSERT INTO events (id, company_id, title, address, starts_at, ends_at, recurrence_months, description, created_by, assigned_to) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, event.ID, user.Company.ID, event.Title, event.Address, event.StartsAt, event.EndsAt, event.RecurrenceMonths, event.Description, user.ID, event.AssignedTo)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create event")
		return
	}
	assignedTo := ""
	if event.AssignedTo != nil {
		assignedTo = *event.AssignedTo
	}
	log.Printf("event created event_id=%s company_id=%s created_by=%s assigned_to=%s starts_at=%s recurrence_months=%d", event.ID, user.Company.ID, user.ID, assignedTo, event.StartsAt.Format(time.RFC3339), event.RecurrenceMonths)
	write(w, http.StatusCreated, event)
}

func nearestOccurrences(event Event, now time.Time) (*Event, *Event) {
	if event.RecurrenceMonths == 0 {
		if event.StartsAt.After(now) {
			return &event, nil
		}
		return nil, &event
	}
	months := (now.Year()-event.StartsAt.Year())*12 + int(now.Month()-event.StartsAt.Month())
	n := months / event.RecurrenceMonths
	if n < 0 {
		n = 0
	}
	duration := event.EndsAt.Sub(event.StartsAt)
	current := event
	current.StartsAt = addMonthsClamped(event.StartsAt, n*event.RecurrenceMonths)
	current.EndsAt = current.StartsAt.Add(duration)
	if current.StartsAt.Before(now) {
		previous := current
		next := event
		next.StartsAt = addMonthsClamped(event.StartsAt, (n+1)*event.RecurrenceMonths)
		next.EndsAt = next.StartsAt.Add(duration)
		return &next, &previous
	}
	next := current
	if n == 0 {
		return &next, nil
	}
	previous := event
	previous.StartsAt = addMonthsClamped(event.StartsAt, (n-1)*event.RecurrenceMonths)
	previous.EndsAt = previous.StartsAt.Add(duration)
	return &next, &previous
}

func (s *Server) addresses(w http.ResponseWriter, r *http.Request, companyID string) {
	rows, err := s.db.Query(r.Context(), eventQuery, companyID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load addresses")
		return
	}
	events, err := scanEvents(rows)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load addresses")
		return
	}
	now := time.Now().UTC()
	byAddress := map[string]*AddressSummary{}
	for _, event := range events {
		summary := byAddress[event.Address]
		if summary == nil {
			summary = &AddressSummary{Address: event.Address}
			byAddress[event.Address] = summary
		}
		next, previous := nearestOccurrences(event, now)
		if next != nil && (summary.NextEvent == nil || next.StartsAt.Before(summary.NextEvent.StartsAt)) {
			summary.NextEvent = next
		}
		if previous != nil && (summary.PreviousEvent == nil || previous.StartsAt.After(summary.PreviousEvent.StartsAt)) {
			summary.PreviousEvent = previous
		}
	}
	result := make([]AddressSummary, 0, len(byAddress))
	for _, summary := range byAddress {
		result = append(result, *summary)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Address) < strings.ToLower(result[j].Address) })
	write(w, http.StatusOK, result)
}

func (s *Server) addressHistory(w http.ResponseWriter, r *http.Request, companyID string) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		fail(w, http.StatusBadRequest, "Choose an address")
		return
	}
	rows, err := s.db.Query(r.Context(), eventQuery+` AND e.address = $2`, companyID, address)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load address history")
		return
	}
	definitions, err := scanEvents(rows)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not load address history")
		return
	}
	now := time.Now().UTC()
	history := []Event{}
	for _, event := range definitions {
		history = append(history, occurrenceStarts(event, event.StartsAt, now)...)
	}
	sort.Slice(history, func(i, j int) bool { return history[i].StartsAt.After(history[j].StartsAt) })
	write(w, http.StatusOK, history)
}

func main() {
	databaseURL, gatewayToken, sessionSecret := os.Getenv("DATABASE_URL"), os.Getenv("BACKEND_API_TOKEN"), os.Getenv("SESSION_SECRET")
	if databaseURL == "" || gatewayToken == "" || sessionSecret == "" {
		log.Fatal("DATABASE_URL, BACKEND_API_TOKEN, and SESSION_SECRET must be set")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	server, err := newServer(ctx, databaseURL, gatewayToken, sessionSecret)
	if err != nil {
		log.Fatal(err)
	}
	defer server.db.Close()
	httpServer := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: server, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("Go API listening on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
