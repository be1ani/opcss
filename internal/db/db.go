// Package db provides database access for OPC file metadata.
package db

import "database/sql"

// SQL DB wrapper
type Store struct{
	db *sql.DB
}

func New(db *sql.DB) *Store{
	return &Store{db: db}
}