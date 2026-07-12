package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountCollectionHandler struct {
	service *service.AccountCollectionService
}

func NewAccountCollectionHandler(s *service.AccountCollectionService) *AccountCollectionHandler {
	return &AccountCollectionHandler{service: s}
}

func accountCollectionError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "account_collection_not_found", "message": "Account collection not found"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "account_collection_error", "message": err.Error()})
}

func (h *AccountCollectionHandler) List(c *gin.Context) {
	items, err := h.service.List(c)
	if err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *AccountCollectionHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Invalid request"})
		return
	}
	item, err := h.service.Create(c, req.Name)
	if err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (h *AccountCollectionHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"message": "Invalid id"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Invalid request"})
		return
	}
	item, err := h.service.Update(c, id, req.Name)
	if err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(200, item)
}
func (h *AccountCollectionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"message": "Invalid id"})
		return
	}
	if err := h.service.Delete(c, id); err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Account collection deleted"})
}
func (h *AccountCollectionHandler) Sort(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Invalid request"})
		return
	}
	if err := h.service.UpdateSort(c, req.IDs); err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Account collections sorted"})
}
func (h *AccountCollectionHandler) ListForAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"message": "Invalid id"})
		return
	}
	items, err := h.service.ListForAccount(c, id)
	if err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(200, items)
}
func (h *AccountCollectionHandler) BatchMembers(c *gin.Context) {
	var req struct {
		AccountIDs    []int64 `json:"account_ids"`
		CollectionIDs []int64 `json:"account_collection_ids"`
		Operation     string  `json:"operation"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Invalid request"})
		return
	}
	if err := h.service.BatchMembers(c, req.AccountIDs, req.CollectionIDs, req.Operation); err != nil {
		accountCollectionError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Account collections updated"})
}
