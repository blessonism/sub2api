package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/announcementread"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"
)

func TestUserAnnouncementReadOrderSortsBeforePagination(t *testing.T) {
	selector := entsql.Select(dbuser.FieldID).
		From(entsql.Table(dbuser.Table)).
		Offset(20).
		Limit(20)
	orders := userListOrder(pagination.PaginationParams{
		SortBy:    "read_at",
		SortOrder: pagination.SortOrderDesc,
	}, service.UserListFilters{ReadStatusAnnouncementID: 42})
	for _, order := range orders {
		order(selector)
	}

	query, args := selector.Query()
	require.Contains(t, query, "LEFT JOIN `"+announcementread.Table+"`")
	require.Contains(t, query, "`t1`.`announcement_id` = ?")
	require.Contains(t, query, "ORDER BY `t1`.`read_at` IS NULL, `t1`.`read_at` DESC, `users`.`id` ASC LIMIT 20 OFFSET 20")
	require.Equal(t, []any{int64(42)}, args)
}
