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
	"github.com/theMagicRabbit/chirpy/internal/database"
)

type apiConfig struct {
	FileserverHits atomic.Int32
	Db *database.Queries
	Env string
}

type validateChirpBody struct {
	Body string `json:"body"`
}

var Profanity = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}


func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

func handleValidateChirp(w http.ResponseWriter, r *http.Request) {
	type validateResponse struct {
		Error string `json:"error"`
		Valid bool   `json:"valid"`
		CleanedBody string `json:"cleaned_body"`
	}
	
	var responseCode int
	responseBody := validateResponse{}

	bodyDecoder := json.NewDecoder(r.Body)
	chirp := validateChirpBody{}
	if err := bodyDecoder.Decode(&chirp); err != nil {
		responseBody.Error = "Something went wrong"
		responseCode = 400
	} else {
		if len(chirp.Body) > 140 {
			responseBody.Error = "Chirp is too long"
			responseCode = 400
		} else {
			message, _ := profanityChecker(chirp.Body)
			responseBody.CleanedBody = message
			responseBody.Valid = true
			responseCode = 200
		}
	}
	data, err := json.Marshal(responseBody)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(responseCode)
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


func (cfg *apiConfig) handleAppHits(w http.ResponseWriter, _ *http.Request) {
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

func (cfg *apiConfig) middlewareNewUser() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Email string `json:"email"`
		}
		var email requestBody
		var userBytes []byte
		bodyDecoder := json.NewDecoder(r.Body)
		if err := bodyDecoder.Decode(&email); err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Error parsing json: %s\n", err.Error())
			return
		}
		id, err := uuid.NewRandom()
		if err != nil {
			w.WriteHeader(500)
			w.Write(userBytes)
			return
		}
		utcTimestamp := time.Now().UTC()
		params := database.CreateUserParams {
			ID: id,
			Email: email.Email,
			CreatedAt: utcTimestamp,
			UpdatedAt: utcTimestamp,
		}
		user, err := cfg.Db.CreateUser(context.Background(), params)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Unable to create new user: %s\n", err.Error())
			return
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

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) resetApp(w http.ResponseWriter, _ *http.Request) {
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
	cfg := apiConfig {
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
	mux.HandleFunc("POST /api/validate_chirp", handleValidateChirp)
	mux.Handle("POST /api/users", cfg.middlewareNewUser())
	mux.HandleFunc("GET /admin/metrics", cfg.handleAppHits)
	mux.HandleFunc("POST /admin/reset", cfg.resetApp)
	server.ListenAndServe()
}
