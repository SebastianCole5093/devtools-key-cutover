package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	keyrotation "github.com/infrai-examples/devtools-key-cutover"
)

func main() {
	client, err := keyrotation.NewClient(os.Getenv("INFRAI_BASE_URL"), os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cutovers", func(w http.ResponseWriter, r *http.Request) {
		var input keyrotation.CutoverInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid JSON input", http.StatusBadRequest)
			return
		}
		result, err := keyrotation.RunCutover(r.Context(), client, input)
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *keyrotation.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				status = apiErr.Status
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("key cutover service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
