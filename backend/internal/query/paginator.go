package query

import (
	"math"
)

// Paginator 查询结果分页器
type Paginator struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// NewPaginator 创建分页器
func NewPaginator(page, pageSize int) *Paginator {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &Paginator{Page: page, PageSize: pageSize}
}

// Offset 计算偏移量
func (p *Paginator) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// TotalPages 计算总页数
func (p *Paginator) TotalPages() int {
	if p.PageSize == 0 {
		return 0
	}
	return int(math.Ceil(float64(p.Total) / float64(p.PageSize)))
}

// HasNext 是否有下一页
func (p *Paginator) HasNext() bool {
	return p.Page < p.TotalPages()
}

// HasPrev 是否有上一页
func (p *Paginator) HasPrev() bool {
	return p.Page > 1
}

// Result 分页结果
type Result struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

// ToResult 转换为分页结果
func (p *Paginator) ToResult(data interface{}) Result {
	return Result{
		Data:       data,
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      p.Total,
		TotalPages: p.TotalPages(),
		HasNext:    p.HasNext(),
		HasPrev:    p.HasPrev(),
	}
}
