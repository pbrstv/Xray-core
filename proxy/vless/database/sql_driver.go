package database

import (
	"database/sql"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type SQLDriver interface {
	Connect(dsn string, poolSize int) (*bun.DB, error)
	IsConnectionError(err error) bool
	IsDuplicateKeyError(err error) bool
	GetType() string
}

type PostgresDriver struct{}

func (p *PostgresDriver) Connect(dsn string, poolSize int) (*bun.DB, error) {
	connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn))
	sqldb := sql.OpenDB(connector)
	sqldb.SetMaxOpenConns(poolSize)
	sqldb.SetMaxIdleConns(poolSize / 2)

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := sqldb.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func (p *PostgresDriver) IsConnectionError(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		// PostgreSQL connection-related error codes
		switch pqErr.Code {
		case "08000", // E_cg: Client cannot be established
			"08003", // E_ct: Connection does not exist
			"08006": // E_cf: Connection failure
			return true
		}
	}
	return false
}

func (p *PostgresDriver) IsDuplicateKeyError(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		// PostgreSQL error code 23505 is unique_violation
		if pqErr.Code == "23505" {
			return true
		}
	}
	return false
}

func (p *PostgresDriver) GetType() string {
	return "PostgreSQL"
}

type MySQLDriver struct{}

func (m *MySQLDriver) Connect(dsn string, poolSize int) (*bun.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	sqldb, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(poolSize)
	sqldb.SetMaxIdleConns(poolSize / 2)

	db := bun.NewDB(sqldb, mysqldialect.New())

	if err := sqldb.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func (m *MySQLDriver) IsConnectionError(err error) bool {
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		// Common MySQL connection error codes
		switch mysqlErr.Number {
		case 1042, // ER_BAD_HOST_ERROR
			1045, // ER_ACCESS_DENIED_ERROR
			1049, // ER_UNKNOWN_COM_ERROR
			2002, // CR_CONNECTION_ERROR
			2003, // CR_CONN_HOST_ERROR
			2005, // CR_UNKNOWN_HOST
			2006, // CR_SERVER_GONE_ERROR
			2013: // CR_SERVER_LOST
			return true
		}
	}
	return false
}

func (m *MySQLDriver) IsDuplicateKeyError(err error) bool {
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		// MySQL error 1062 is ER_DUP_ENTRY (Duplicate entry)
		if mysqlErr.Number == 1062 {
			return true
		}
	}
	return false
}

func (m *MySQLDriver) GetType() string {
	return "MySQL"
}
