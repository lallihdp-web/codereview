package testdata

import (
	"context"
	"database/sql"
)

// UserRepositoryGood demonstrates proper database patterns
type UserRepositoryGood struct {
	db *sql.DB
}

// GetUsersWithOrders demonstrates proper JOIN usage instead of N+1
func (r *UserRepositoryGood) GetUsersWithOrders(ctx context.Context) ([]User, error) {
	// GOOD: Single query with JOIN instead of N+1
	query := `
		SELECT u.id, u.name, u.email, o.id, o.user_id, o.total
		FROM users u
		LEFT JOIN orders o ON u.id = o.user_id
		ORDER BY u.id
		LIMIT 100
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userMap := make(map[int]*User)
	var users []User

	for rows.Next() {
		var u User
		var o Order
		var orderID sql.NullInt64

		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &orderID, &o.UserID, &o.Total); err != nil {
			return nil, err
		}

		if existing, ok := userMap[u.ID]; ok {
			if orderID.Valid {
				o.ID = int(orderID.Int64)
				existing.Orders = append(existing.Orders, o)
			}
		} else {
			if orderID.Valid {
				o.ID = int(orderID.Int64)
				u.Orders = []Order{o}
			}
			userMap[u.ID] = &u
			users = append(users, u)
		}
	}

	return users, nil
}

// GetUserStats demonstrates combining multiple aggregations in one query
func (r *UserRepositoryGood) GetUserStats(ctx context.Context, userID int) (*UserStats, error) {
	// GOOD: Single query with multiple aggregations instead of multiple trips
	query := `
		SELECT
			COUNT(*) as order_count,
			COALESCE(SUM(total), 0) as total_spent,
			COALESCE(AVG(total), 0) as avg_order_value,
			MAX(created_at) as last_order_date
		FROM orders
		WHERE user_id = $1
	`
	var stats UserStats
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.OrderCount,
		&stats.TotalSpent,
		&stats.AvgOrderValue,
		&stats.LastOrderDate,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetUsersByIDs demonstrates batch query with IN clause
func (r *UserRepositoryGood) GetUsersByIDs(ctx context.Context, ids []int) ([]User, error) {
	// GOOD: Batch query with IN clause instead of loop
	query := `SELECT id, name, email FROM users WHERE id = ANY($1) LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query, ids)
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
		users = append(users, u)
	}

	return users, nil
}

// SearchUsers demonstrates proper LIKE usage with suffix wildcard
func (r *UserRepositoryGood) SearchUsers(ctx context.Context, prefix string) ([]User, error) {
	// GOOD: Suffix wildcard allows index usage
	query := `SELECT id, name, email FROM users WHERE name LIKE $1 LIMIT 50`
	rows, err := r.db.QueryContext(ctx, query, prefix+"%")
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
		users = append(users, u)
	}

	return users, nil
}

// DeleteUser demonstrates safe DELETE with WHERE clause
func (r *UserRepositoryGood) DeleteUser(ctx context.Context, userID int) error {
	// GOOD: DELETE with WHERE clause
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	return err
}
