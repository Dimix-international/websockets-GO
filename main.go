package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/Dimix-international/websockets-GO/internal/service"
)

func main() {
	// Create a root ctx and a CancelFunc which can be used to cancel retentionMap goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupAPI(ctx)

	// if err := http.ListenAndServeTLS(":8080", "server.crt", "server.key", nil); err != nil {
	// 	log.Fatal("ListenAndServe: ", err)
	// }
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupAPI(ctx context.Context) {
	managerAPI := service.NewManager(ctx)

	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/ws", managerAPI.ServeWS)
	http.HandleFunc("/login", managerAPI.LoginHandler)
	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, len(managerAPI.Clients))
	})
}
