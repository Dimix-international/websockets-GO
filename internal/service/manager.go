package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	/**
	websocketUpgrader is used to upgrade incomming HTTP requests into a persitent websocket connection
	*/
	websocketUpgrader = websocket.Upgrader{
		// Apply the Origin Checker
		CheckOrigin:     checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ErrEventNotSupported = errors.New("this event type is not supported")
)

// Manager is used to hold references to all Clients Registered, and Broadcasting etc
type Manager struct {
	mu      sync.RWMutex
	Clients ClientList
	// handlers are functions that are used to handle Events
	Handlers map[string]EventHandler
	// otps is a map of allowed OTP to accept connections from
	otps RetentionMap
}

// NewManager is used to initalize all the values inside the manage
func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		Clients:  make(ClientList),
		Handlers: make(map[string]EventHandler, 2),
		// Create a new retentionMap that removes Otps older than 5 seconds
		otps: NewRetentionMap(ctx, 5*time.Second),
	}
	m.setupEventHandlers()
	return m
}

// checkOrigin will check origin and return true if its allowed
func checkOrigin(r *http.Request) bool {
	// Grab the request origin
	origin := r.Header.Get("Origin")
	//"https://localhost:8080" if crypt
	return origin == "http://localhost:8080"
}

// setupEventHandlers configures and adds all handlers
func (m *Manager) setupEventHandlers() {
	m.Handlers[EventSendMessage] = SendMessageHandler
	m.Handlers[EventChangeRoom] = ChatRoomHandler
}

// RouteEvent is used to make sure the correct event goes into the correct handler
func (m *Manager) routeEvent(event Event, c *Client) error {
	// Check if Handler is present in Map
	if handler, ok := m.Handlers[event.Type]; ok {
		// Execute the handler and return any err
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	}

	return ErrEventNotSupported
}

// serveWS is a HTTP Handler that the has the Manager that allows connections
func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Grab the OTP in the Get param
	otp := r.URL.Query().Get("otp")
	if otp == "" {
		// Tell the user its not authorized
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Verify OTP is existing
	if !m.otps.VerifyOTP(otp) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Println("New connection")
	// Begin by upgrading the HTTP request
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := NewClient(conn, m)
	m.addClient(client)

	go client.readMessages()
	go client.writeMessages()
}

// addClient will add clients to our clientList
func (m *Manager) addClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add Client
	m.Clients[client] = true
}

// removeClient will remove the client and clean up
func (m *Manager) removeClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if Client exists, then delete it
	if _, ok := m.Clients[client]; ok {
		// close connection
		client.connection.Close()
		// remove
		delete(m.Clients, client)
	}
}

// LoginHandler is used to verify an user authentication and return a one time password
func (m *Manager) LoginHandler(w http.ResponseWriter, r *http.Request) {
	type userLoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req userLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Authenticate user / Verify Access token, what ever auth method you use
	if req.Username == "percy" && req.Password == "123" {
		// format to return otp in to the frontend
		type response struct {
			OTP string `json:"otp"`
		}

		// add a new OTP
		otp := m.otps.NewOTP()
		resp := response{
			OTP: otp.Key,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			return
		}

		// Return a response to the Authenticated user with the OTP
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return

	}
	// Failure to auth
	w.WriteHeader(http.StatusUnauthorized)
}
