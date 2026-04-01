package src

import (
	"log"
	"net/http"
	"strconv"
)

const DefaultPort = 1355

func StartServer(port int) error {
	return http.ListenAndServe(":" + strconv.Itoa(port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("portless-go proxy running")); err != nil {
			log.Printf("write error: %v", err)
		}
	}))

}