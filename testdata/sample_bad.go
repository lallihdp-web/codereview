package testdata

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"
)

// UserRepository demonstrates various database anti-patterns
type UserRepository struct {
	db   *sql.DB
	Psql squirrel.StatementBuilderType
}

// GetUsersWithOrders demonstrates N+1 query problem
func (r *UserRepository) GetUsersWithOrders(ctx context.Context) ([]User, error) {
	// First query to get all users
	rows, err := r.db.QueryContext(ctx, "SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}

		// N+1 PROBLEM: Query inside loop!
		orderRows, err := r.db.QueryContext(ctx, "SELECT * FROM orders WHERE user_id = ?", u.ID)
		if err != nil {
			return nil, err
		}

		for orderRows.Next() {
			var o Order
			orderRows.Scan(&o.ID, &o.UserID, &o.Total)
			u.Orders = append(u.Orders, o)
		}
		orderRows.Close()

		users = append(users, u)
	}

	return users, nil
}

// GetUserStats demonstrates multiple database trips
func (r *UserRepository) GetUserStats(ctx context.Context, userID int) (*UserStats, error) {
	var stats UserStats

	// Multiple trips that could be combined
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = ?", userID).Scan(&stats.OrderCount)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx, "SELECT SUM(total) FROM orders WHERE user_id = ?", userID).Scan(&stats.TotalSpent)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx, "SELECT AVG(total) FROM orders WHERE user_id = ?", userID).Scan(&stats.AvgOrderValue)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx, "SELECT MAX(created_at) FROM orders WHERE user_id = ?", userID).Scan(&stats.LastOrderDate)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// DeleteAllUsers demonstrates dangerous SQL pattern
func (r *UserRepository) DeleteAllUsers(ctx context.Context) error {
	// WARNING: DELETE without WHERE
	_, err := r.db.ExecContext(ctx, "DELETE FROM users")
	return err
}

// SearchUsers demonstrates inefficient LIKE pattern
func (r *UserRepository) SearchUsers(ctx context.Context, query string) ([]User, error) {
	// WARNING: Leading wildcard prevents index usage
	rows, err := r.db.QueryContext(ctx, "SELECT * FROM users WHERE name LIKE '%"+query+"%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Name, &u.Email)
		users = append(users, u)
	}
	return users, nil
}

// GetAllProducts demonstrates SELECT * anti-pattern
func (r *UserRepository) GetAllProducts(ctx context.Context) ([]Product, error) {
	// WARNING: SELECT * without LIMIT
	rows, err := r.db.QueryContext(ctx, "SELECT * FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		rows.Scan(&p.ID, &p.Name, &p.Price)
		products = append(products, p)
	}
	return products, nil
}

// NestedLoopQuery demonstrates nested N+1 problem (even worse!)
func (r *UserRepository) NestedLoopQuery(ctx context.Context) error {
	categories, _ := r.db.QueryContext(ctx, "SELECT id FROM categories")
	defer categories.Close()

	for categories.Next() {
		var catID int
		categories.Scan(&catID)

		// First level of N+1
		products, _ := r.db.QueryContext(ctx, "SELECT id FROM products WHERE category_id = ?", catID)

		for products.Next() {
			var prodID int
			products.Scan(&prodID)

			// Second level of N+1 (nested)!
			r.db.QueryContext(ctx, "SELECT * FROM reviews WHERE product_id = ?", prodID)
		}
		products.Close()
	}

	return nil
}

// Types for the example
type User struct {
	ID     int
	Name   string
	Email  string
	Orders []Order
}

type Order struct {
	ID     int
	UserID int
	Total  float64
}

type UserStats struct {
	OrderCount    int
	TotalSpent    float64
	AvgOrderValue float64
	LastOrderDate string
}

type Product struct {
	ID    int
	Name  string
	Price float64
}
