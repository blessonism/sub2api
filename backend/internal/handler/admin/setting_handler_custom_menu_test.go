//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsNormalizesBlankCustomMenuOpenMode(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCustomMenuItems: "[]",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{
				"label":      "无限画布",
				"url":        "https://canvas.example/app",
				"visibility": "user",
				"sort_order": 0,
			},
		},
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var items []dto.CustomMenuItem
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyCustomMenuItems]), &items))
	require.Len(t, items, 1)
	require.Equal(t, dto.CustomMenuOpenModeEmbed, items[0].OpenMode)
	require.Equal(t, "https://canvas.example/app", items[0].URL)
}

func TestUpdateSettingsPersistsExternalCustomMenuOpenMode(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCustomMenuItems: "[]",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{
				"label":      "无限画布",
				"url":        "https://canvas.example/app",
				"visibility": "user",
				"open_mode":  "external",
				"sort_order": 0,
			},
		},
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var items []dto.CustomMenuItem
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyCustomMenuItems]), &items))
	require.Len(t, items, 1)
	require.Equal(t, dto.CustomMenuOpenModeExternal, items[0].OpenMode)
}

func TestUpdateSettingsRejectsInvalidCustomMenuOpenMode(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCustomMenuItems: "[]",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{
				"label":      "无限画布",
				"url":        "https://canvas.example/app",
				"visibility": "user",
				"open_mode":  "popup",
				"sort_order": 0,
			},
		},
	}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdateSettingsRejectsExternalMarkdownCustomMenu(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCustomMenuItems: "[]",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{
				"label":      "帮助",
				"url":        "md:help",
				"visibility": "user",
				"open_mode":  "external",
				"sort_order": 0,
			},
		},
	}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
