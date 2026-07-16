package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAffiliateLeaderboardQueryKeepsGlobalRankAndSeparatesAggregates(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateLeaderboardCTE+affiliateLeaderboardSearch), " ")
	require.Contains(t, query, "SUM(invitee.total_recharged)")
	require.Contains(t, query, "SUM(rc.value)")
	require.Contains(t, query, "rc.status = 'used'")
	require.Contains(t, query, "rc.type = 'balance'")
	require.Contains(t, query, "ROW_NUMBER() OVER")
	require.Contains(t, query, "inviter_aff.aff_count DESC")
	require.Contains(t, query, "payment_redeem.payment_redeem_amount, 0) DESC")
	require.Contains(t, query, "inviter_aff.aff_count > 0")
	require.Contains(t, query, "inviter.deleted_at IS NULL")
	require.NotContains(t, query, "payment_orders")
	require.True(t, strings.Index(query, "ROW_NUMBER() OVER") < strings.Index(query, "WHERE $1 = ''"))
}

func TestAffiliateRepositoryListLeaderboardScansStableRanks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client}

	mock.ExpectQuery(regexp.QuoteMeta("WITH invitee_credit AS (")).
		WithArgs("alice", "%alice%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("WITH invitee_credit AS (")).
		WithArgs("alice", "%alice%", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"rank", "user_id", "email", "username", "aff_code", "invite_count", "all_credit_amount", "payment_redeem_amount",
		}).AddRow(7, 42, "alice@example.com", "Alice", "ALICE42", 3, 120.5, 88.0))

	items, total, err := repo.ListAffiliateLeaderboard(context.Background(), service.AffiliateAdminFilter{
		Search: " alice ", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []service.AffiliateLeaderboardEntry{{
		Rank: 7, UserID: 42, Email: "alice@example.com", Username: "Alice", AffCode: "ALICE42",
		InviteCount: 3, AllCreditAmount: 120.5, PaymentRedeemAmount: 88,
	}}, items)
	require.NoError(t, mock.ExpectationsWereMet())
}
