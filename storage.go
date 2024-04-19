package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Storage interface {
	CreateAccount(*Account) error
	DeleteAccount(int) error
	UpdateAccount(*Account) error
	GetAccounts() ([]*Account, error)
	GetAccountByID(int) (*Account, error)
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	connStr := "user=postgres dbname=postgres password=gobank sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{
		db: db,
	}, nil
}

func (s *PostgresStore) Init() error {
	return s.createAccountTable()
}

func (s *PostgresStore) createAccountTable() error {
	query := `CREATE TABLE IF NOT EXISTS account (
		id SERIAL PRIMARY KEY,
		firstName VARCHAR(50),
		lastName VARCHAR(50),
		number SERIAL,
		balance FLOAT,
		createdAt TIMESTAMP
	)`

	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) CreateAccount(acc *Account) error {
	query := `
	INSERT
	INTO 
	account (firstName, lastName, number , balance, createdAt) 
	VALUES (
		$1, $2, $3, $4, $5
	)`

	resp, err := s.db.Query(query, acc.FirstName, acc.LastName, acc.Number, acc.Balance, acc.CreatedAt)
	if err != nil {
		return err
	}
	fmt.Printf("%v+", resp)
	return nil
}

func (s *PostgresStore) UpdateAccount(*Account) error {
	return nil
}

func (s *PostgresStore) DeleteAccount(id int) error {
	_, err := s.db.Query(`DELETE FROM account WHERE id = $1`, int(id))
	return err
}
func (s *PostgresStore) GetAccountByID(id int) (*Account, error) {
	rows, err := s.db.Query(`SELECT * FROM account WHERE id =$1
	`, int(id))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		return scanIntoAccounts(rows)
	}
	return nil, fmt.Errorf("Account %d not found!!", id)
}

func (s *PostgresStore) GetAccounts() ([]*Account, error) {
	rows, err := s.db.Query("SELECT * FROM account")
	if err != nil {
		return nil, err
	}
	defer rows.Close() // Close the rows when done to free resources

	accounts := []*Account{}
	for rows.Next() {
		account, err := scanIntoAccounts(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil { // Check for any error during iteration
		return nil, err
	}

	return accounts, nil
}

func scanIntoAccounts(rows *sql.Rows) (*Account, error) {
	// account := &Account{} // Initialize a new Account struct
	account := new(Account)
	err := rows.Scan(
		&account.ID,
		&account.FirstName,
		&account.LastName,
		&account.Number,
		&account.Balance,
		&account.CreatedAt,
	)
	return account, err
}
