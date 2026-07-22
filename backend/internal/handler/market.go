package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
)

type ModuleLister interface {
	ListModules(query, category, sort string, page, perPage int) ([]*domain.MarketModule, int)
	GetModule(slugOrID string) (*domain.MarketModule, error)
	StarModule(slugOrID string) (int, error)
	AddReview(moduleID, uid, username string, rating int, comment string) error
	GetReviews(moduleID string) []*domain.MarketReview
	PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error)
	TrendingModules(limit int) []*domain.MarketModule
	Categories() []string
}

type MarketHandler struct {
	market ModuleLister
}

func NewMarketHandler(market ModuleLister) *MarketHandler {
	return &MarketHandler{market: market}
}

// GET /market/modules?query=X&category=Y&sort=Z&page=1&per_page=20
func (h *MarketHandler) ListModules(c fiber.Ctx) error {
	query := c.Query("query")
	category := c.Query("category")
	sortBy := c.Query("sort", "stars")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	modules, total := h.market.ListModules(query, category, sortBy, page, perPage)
	return c.JSON(fiber.Map{"modules": modules, "total": total, "page": page, "per_page": perPage})
}

// GET /market/module/:slug
func (h *MarketHandler) GetModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	mod, err := h.market.GetModule(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mod)
}

// POST /market/module/:slug/star
func (h *MarketHandler) StarModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	stars, err := h.market.StarModule(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"stars": stars})
}

// POST /market/module/:slug/review
func (h *MarketHandler) AddReview(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		UID      string `json:"uid"`
		Username string `json:"username"`
		Rating   int    `json:"rating"`
		Comment  string `json:"comment"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.market.AddReview(slug, req.UID, req.Username, req.Rating, req.Comment); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

// GET /market/module/:slug/reviews
func (h *MarketHandler) GetReviews(c fiber.Ctx) error {
	slug := c.Params("slug")
	reviews := h.market.GetReviews(slug)
	return c.JSON(fiber.Map{"reviews": reviews})
}

// POST /market/publish
func (h *MarketHandler) Publish(c fiber.Ctx) error {
	var mod domain.MarketModule
	if err := c.Bind().JSON(&mod); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	result, err := h.market.PublishModule(&mod)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GET /market/trending?limit=10
func (h *MarketHandler) Trending(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 {
		limit = 10
	}
	modules := h.market.TrendingModules(limit)
	return c.JSON(fiber.Map{"modules": modules})
}

// GET /market/categories
func (h *MarketHandler) Categories(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"categories": h.market.Categories()})
}
