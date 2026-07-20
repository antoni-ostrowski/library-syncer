package db

import (
	"database/sql"
)

type DbService struct {
	db *sql.DB
}

func NewDbService(db *sql.DB) *DbService {
	return &DbService{db: db}
}
