package main

import (
	"fmt"

	"github.com/zon/chat/core/pg"
	"gorm.io/gorm"
)

// Context provides shared dependencies for wurbctl commands.
type Context struct {
	db *gorm.DB
}

// DB returns the shared database connection, opening it if necessary.
func (c *Context) DB() (*gorm.DB, error) {
	if c.db != nil {
		return c.db, nil
	}

	db, err := pg.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	c.db = db
	return c.db, nil
}

// Close closes the shared database connection if it was opened.
func (c *Context) Close() error {
	if c.db == nil {
		return nil
	}

	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
