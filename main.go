package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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
	body := fmt.Sprintf(htmlPage, cfg.fileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) resetHitCounter(w http.ResponseWriter, _ *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

func main() {
	cfg := apiConfig{}
	mux := http.NewServeMux()
	fileServerHander := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", cfg.middlewareMetricsInc(fileServerHander))
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	mux.HandleFunc("GET /api/healthz", handleHealthz)
	mux.HandleFunc("POST /api/validate_chirp", handleValidateChirp)
	mux.HandleFunc("GET /admin/metrics", cfg.handleAppHits)
	mux.HandleFunc("POST /admin/reset", cfg.resetHitCounter)
	server.ListenAndServe()
}
