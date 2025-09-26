package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/theMagicRabbit/chirpy/internal/auth"
	"github.com/theMagicRabbit/chirpy/internal/database"
)

type ApiConfig struct {
	FileserverHits atomic.Int32
	Db *database.Queries
	Env string
}

var Profanity = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}

type User struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string `json:"email"`
}

type Chirp struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body string `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

func (cfg *ApiConfig) handleChirps(w http.ResponseWriter, r *http.Request) {
	type validateResponse struct {
		Error string `json:"error"`
		Valid bool   `json:"valid"`
		CleanedBody string `json:"cleaned_body"`
	}

	type chirpReq struct {
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	
	bodyDecoder := json.NewDecoder(r.Body)
	chirp := chirpReq{}
	if err := bodyDecoder.Decode(&chirp); err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to decode json string: %s\n", err.Error())
		return
	}
	if len(chirp.Body) > 140 {
		w.WriteHeader(400)
		fmt.Fprintln(w, "Chirp is too long")
		return
	}
	if message, wasCleaned := profanityChecker(chirp.Body); wasCleaned {
		chirp.Body = message
	}
	chirpID, err := uuid.NewRandom()
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Unable to create UUID: %s\n", err.Error())
		return
	}
	utcTimestamp := time.Now().UTC()
	params := database.CreateChirpParams {
		ID: chirpID,
		CreatedAt: utcTimestamp,
		UpdatedAt: utcTimestamp,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}
	dbChirp, err := cfg.Db.CreateChirp(context.Background(), params)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Unable to store chirp: %s\n", err.Error())
		return
	}
	localChirp := Chirp{
		ID: dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.CreatedAt,
		Body: dbChirp.Body,
		UserID: dbChirp.UserID,
	}
	data, err := json.Marshal(localChirp)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error unmarshaling chirp: %s\n", err.Error())
		return
	}
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(201)
	w.Write(data)
}

// Returns message string and had_profanity boolean. had_profanity is true if any replacements were
// performed and false otherwise.
func profanityChecker(message string) (string, bool) {
	profanityReplacement := "****"
	words := strings.Fields(message)
	hadProfanity := false
	cleanWords := []string{}
	for _, w := range words {
		if slices.Contains(Profanity, strings.ToLower(w)) {
			cleanWords = append(cleanWords, profanityReplacement)
			hadProfanity = true
		} else {
			cleanWords = append(cleanWords, w)
		}
	}
	if hadProfanity {
		return strings.Join(cleanWords, " "), hadProfanity
	}
	return message, hadProfanity
}


func (cfg *ApiConfig) handleAppHits(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	htmlPage := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>
`
	body := fmt.Sprintf(htmlPage, cfg.FileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *ApiConfig) middlewareNewUser() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Email string `json:"email"`
			Password string `json:"password"`
		}
		var userRequest requestBody
		var userBytes []byte
		bodyDecoder := json.NewDecoder(r.Body)
		if err := bodyDecoder.Decode(&userRequest); err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Error decoding request: %s\n", err.Error())
			return
		}
		if userRequest.Password == "" {
			w.WriteHeader(400)
			fmt.Fprintln(w, "Password is required")
			return
		}
		id, err := uuid.NewRandom()
		if err != nil {
			w.WriteHeader(500)
			w.Write(userBytes)
			return
		}
		utcTimestamp := time.Now().UTC()
		hashedPassword, err := auth.HashPassword(userRequest.Password)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, "Unable to hash password")
			return
		}
		params := database.CreateUserParams {
			ID: id,
			Email: userRequest.Email,
			CreatedAt: utcTimestamp,
			UpdatedAt: utcTimestamp,
			HashedPassword: hashedPassword,
		}
		databaseUser, err := cfg.Db.CreateUser(context.Background(), params)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Unable to create new user: %s\n", err.Error())
			return
		}
		user := User{
			ID: databaseUser.ID,
			CreatedAt: databaseUser.CreatedAt,
			UpdatedAt: databaseUser.UpdatedAt,
			Email: databaseUser.Email,
		}
		userBytes, err = json.Marshal(&user)
		if err != nil {
			w.WriteHeader(500)
			w.Write(userBytes)
			return
		}
		w.WriteHeader(201)
		w.Write(userBytes)
	})

}

func (cfg *ApiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *ApiConfig) resetApp(w http.ResponseWriter, _ *http.Request) {
	if cfg.Env != "dev" {
		w.WriteHeader(403)
		return
	}
	if err := cfg.Db.DeleteAllUsers(context.Background()); err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error deleting users: %s\n", err.Error())
		return
	}
	cfg.FileserverHits.Store(0)
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

func (cfg *ApiConfig) handleGetAllChirps(w http.ResponseWriter, _ *http.Request) {
	chirps, err := cfg.Db.GetAllChirps(context.Background())
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error getting data from db: %s\n", err.Error())
		return
	}
	data, err := json.Marshal(chirps)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error marshaling json: %s\n", err.Error())
		return
	}
	w.Header().Add("content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *ApiConfig) handleGetChirpByID(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(idString)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to parse userID: %s\n", err.Error())
		return
	}
	chirp, err := cfg.Db.GetChirpByID(context.Background(), chirpID)
	if err != nil {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Chirp not found in database: %s\n", err.Error())
		return
	}
	data, err := json.Marshal(chirp)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error marshaling json: %s\n", err.Error())
		return
	}
	w.Header().Add("content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *ApiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type userRequest struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	userDecoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	userBody := userRequest{}
	if err := userDecoder.Decode(&userBody); err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Error marshaling json: %s\n", err.Error())
		return
	}
	user := User{}
	if localUser, err := cfg.Db.GetUserByEmail(context.Background(), userBody.Email); err != nil {
		w.WriteHeader(401)
		return
	} else {
		if match, err2 := auth.CheckPasswordHash(userBody.Password, localUser.HashedPassword); err2 != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "cannot validate password: %s\n", err2.Error())
			return
		} else if !match {
			w.WriteHeader(401)
			return
		}
		user.ID = localUser.ID
		user.Email = localUser.Email
		user.CreatedAt = localUser.CreatedAt
		user.UpdatedAt = localUser.UpdatedAt
	}
	if data, err := json.Marshal(user); err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "cannot marshal json: %s\n", err.Error())
		return
	} else {
	 	w.WriteHeader(200)
	 	w.Write(data)
	}
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platformEnv := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	dbQueries := database.New(db)
	cfg := ApiConfig {
		Db: dbQueries,
		Env: platformEnv,
	}
	mux := http.NewServeMux()
	fileServerHander := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", cfg.middlewareMetricsInc(fileServerHander))
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	mux.HandleFunc("GET /api/healthz", handleHealthz)
	mux.HandleFunc("POST /api/chirps", cfg.handleChirps)
	mux.HandleFunc("GET /api/chirps", cfg.handleGetAllChirps)
	mux.HandleFunc("POST /api/login", cfg.handleLogin)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handleGetChirpByID)
	mux.Handle("POST /api/users", cfg.middlewareNewUser())
	mux.HandleFunc("GET /admin/metrics", cfg.handleAppHits)
	mux.HandleFunc("POST /admin/reset", cfg.resetApp)
	server.ListenAndServe()
}
