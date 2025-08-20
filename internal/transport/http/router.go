package rest

import (
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	service *usecase.OrderService
	log     ports.Logger
}

func NewHandler(service *usecase.OrderService, log ports.Logger) *Handler {
	return &Handler{service: service, log: log}
}

func NewRouter(h *Handler, staticDir string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requsetLogger(h.log))

	r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/order/:id", h.getOrderByID)
	r.GET("/customer/:id/orders", h.listOrdersByCustomer)

	if staticDir != "" {
		r.Static("/static", staticDir)
		r.StaticFile("/", filepath.Join(staticDir, "index.html"))
	}

	return r
}

func (h *Handler) getOrderByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty id"})
		return
	}
	order, err := h.service.GetOrder(c.Request.Context(), id)
	if err != nil {
		h.log.Errorf(c.Request.Context(), "GetOrder failed id=%s err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *Handler) listOrdersByCustomer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty customer id"})
		return
	}

	// limit/offset с безопасными дефолтами и границами
	limit := 20
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && v >= 0 {
		offset = v
	}

	orders, err := h.service.OrdersByCustomer(c.Request.Context(), id, limit, offset)
	if err != nil {
		h.log.Errorf(c.Request.Context(), "OrdersByCustomer failed id=%s err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func requsetLogger(log ports.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Infof(c.Request.Context(), "request method=%s path=%s status=%d duration=%s", c.Request.Method, c.FullPath(), c.Writer.Status(), duration)
	}
}