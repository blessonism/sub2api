//go:build unit

package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestChannelPricingIntervals_NullableTierLabel(t *testing.T) {
	for _, table := range []string{"channel_pricing_intervals", "channel_account_stats_pricing_intervals"} {
		t.Run(table, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			repo := &channelRepository{db: db}
			load := repo.batchLoadIntervals
			withMultipliers := table == "channel_pricing_intervals"
			if !withMultipliers {
				load = repo.batchLoadAccountStatsIntervals
			}

			columns := []string{
				"id", "pricing_id", "min_tokens", "max_tokens", "tier_label",
				"input_price", "output_price", "cache_write_price", "cache_write_1h_price", "cache_read_price",
			}
			if withMultipliers {
				columns = append(columns, "input_multiplier", "output_multiplier", "cache_write_multiplier", "cache_read_multiplier")
			}
			columns = append(columns, "per_request_price", "sort_order", "created_at", "updated_at")
			rows := sqlmock.NewRows(columns)
			now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
			for i, label := range []driver.Value{nil, "", "2K"} {
				values := []driver.Value{
					int64(i + 1), int64(11), int64(i * 100), nil, label,
					float64(0), float64(1.25e-6), nil, nil, nil,
				}
				if withMultipliers {
					values = append(values, float64(2), nil, nil, nil)
				}
				rows.AddRow(append(values, nil, int64(i), now, now)...)
			}
			mock.ExpectQuery("FROM " + table).
				WithArgs(pq.Array([]int64{11})).
				WillReturnRows(rows)

			intervals, err := load(context.Background(), []int64{11})
			require.NoError(t, err)
			require.Len(t, intervals, 1)
			require.Len(t, intervals[11], 3)
			for i, label := range []string{"", "", "2K"} {
				iv := intervals[11][i]
				require.Equal(t, int64(i+1), iv.ID)
				require.Equal(t, int64(11), iv.PricingID)
				require.Equal(t, i*100, iv.MinTokens)
				require.Nil(t, iv.MaxTokens)
				require.Equal(t, label, iv.TierLabel)
				require.NotNil(t, iv.InputPrice)
				require.Zero(t, *iv.InputPrice)
				require.NotNil(t, iv.OutputPrice)
				require.Equal(t, 1.25e-6, *iv.OutputPrice)
				require.Nil(t, iv.CacheWritePrice)
				require.Nil(t, iv.CacheWrite1hPrice)
				require.Nil(t, iv.CacheReadPrice)
				require.Nil(t, iv.PerRequestPrice)
				if withMultipliers {
					require.NotNil(t, iv.InputMultiplier)
					require.Equal(t, 2.0, *iv.InputMultiplier)
				} else {
					require.Nil(t, iv.InputMultiplier)
				}
				require.Equal(t, i, iv.SortOrder)
				require.Equal(t, now, iv.CreatedAt)
				require.Equal(t, now, iv.UpdatedAt)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
