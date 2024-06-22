package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type APIServer struct {
	listenAddr string
	store      Storage
}

// Constructor
func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := http.NewServeMux()

	// Route Handlers
	router.HandleFunc("POST /login", makeHTTPhandleFunc(s.handleLogin))
	router.HandleFunc("GET /accounts", makeHTTPhandleFunc(s.handleGetAccount))
	router.HandleFunc("POST /accounts", makeHTTPhandleFunc(s.handleCreateAccount))
	router.HandleFunc("GET /accounts/{id}", withJWTAuth(makeHTTPhandleFunc(s.handleGetAccountByID), s.store))
	router.HandleFunc("DELETE /accounts/{id}", makeHTTPhandleFunc(s.handleGetAccountByID))
	router.HandleFunc("POST /transfer", makeHTTPhandleFunc(s.handleTransferMoney))

	log.Println("API Server running on port", s.listenAddr)
	// Listen and run Server
	err := http.ListenAndServe(s.listenAddr, router)
	if err != nil {
		log.Fatal(err)
	}
}

// func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
// 	if r.Method == "GET" {
// 		return s.handleGetAccount(w, r)
// 	}
// 	if r.Method == "POST" {
// 		return s.handleCreateAccount(w, r)
// 	}
// 	return fmt.Errorf("method not allowed %s", r.Method)
// }

// 462840

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	// search the user
	acc, err := s.store.GetAccountByNumber(req.Number)
	if err != nil {
		return err
	}
	if !acc.ValidPassword(req.Password) {
		return fmt.Errorf("not authenticated")
	}

	token, err := createJWT(acc)
	if err != nil {
		return err
	}
	resp := LoginResponse{Token: token, Number: int(acc.Number)}
	return writeJSON(w, http.StatusOK, resp)
}
func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, accounts)
}
func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		id, err := getIdFromRequest(r)
		if err != nil {
			return err
		}
		account, err := s.store.GetAccountByID(id)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, account)
	}
	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}
	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	//  ? Another way of writing this below
	// createAccountReq := CreateAccountRequest{}
	// if err := json.NewDecoder(r.Body).Decode(&createAccountReq); err != nil {
	// 	return err
	// }
	// return nil

	creatAcctReq := new(CreateAccountRequest)
	// fmt.Println(creatAcctReq)

	if err := json.NewDecoder(r.Body).Decode(creatAcctReq); err != nil {
		return err
	}
	defer r.Body.Close()
	account, err := NewAccount(creatAcctReq.FirstName, creatAcctReq.LastName, creatAcctReq.Password)
	if err != nil {
		return err
	}
	if err := s.store.CreateAccount(account); err != nil {
		return err
	}
	tokenString, err := createJWT(account)
	if err != nil {
		return err
	}
	fmt.Println("JWT token:", tokenString)
	return writeJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getIdFromRequest(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, map[string]int{"deleted": id})

}

func (s *APIServer) handleTransferMoney(w http.ResponseWriter, r *http.Request) error {
	transferReq := new(TransferRequest)
	if err := json.NewDecoder(r.Body).Decode(transferReq); err != nil {
		return err
	}
	defer r.Body.Close()

	return writeJSON(w, http.StatusOK, transferReq)
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Set("Content-type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func createJWT(account *Account) (string, error) {
	// Create the Claims
	claims := &jwt.MapClaims{
		"expiresAt":     jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		"accountNumber": account.Number,
	}
	secret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(secret))

	return ss, err
}

func permissionDenied(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, APiError{Error: "Permission Denied"})
}

// More like a middleware
func withJWTAuth(handlerFunc http.HandlerFunc, store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Calling JWT Auth middleware")
		tokenString := r.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenString)
		if err != nil {
			permissionDenied(w)
			return
		}

		if !token.Valid {
			permissionDenied(w)
			return
		}
		userID, err := getIdFromRequest(r)
		if err != nil {
			permissionDenied(w)
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		account, err := store.GetAccountByID(userID)

		if account.Number != int64(claims["accountNumber"].(float64)) {
			permissionDenied(w)
			return
		}
		if err != nil {
			permissionDenied(w)
		}

		fmt.Println(claims)

		handlerFunc(w, r)
	}
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	fmt.Println(secret)
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(secret), nil
	})
}

// ?Important Functions
type apiFunc func(http.ResponseWriter, *http.Request) error
type APiError struct {
	Error string `json:"error"`
}

func makeHTTPhandleFunc(function apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := function(w, r); err != nil {
			// Handle the Error
			writeJSON(w, http.StatusBadRequest, APiError{Error: err.Error()})
		}
	}
}

func getIdFromRequest(req *http.Request) (int, error) {
	// idStr := mux.Vars(r)["id"]
	idStr := req.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return id, fmt.Errorf("invalid id given %s", idStr)
	}
	return id, nil
}
