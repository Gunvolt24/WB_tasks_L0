package rest

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Handler — обработчик HTTP-запросов.
type Handler struct {
	service    ports.OrderReadService
	log        ports.Logger
	reqTimeout time.Duration // таймаут на обработку запроса
}

// NewHandler — DI-конструктор. Если reqTimeout <= 0, ставим дефолт 3s.
func NewHandler(service ports.OrderReadService, log ports.Logger, reqTimeout time.Duration) *Handler {
	if reqTimeout <= 0 {
		reqTimeout = 3 * time.Second
	}
	return &Handler{service: service, log: log, reqTimeout: reqTimeout}
}

// NewRouter — сборка роутера gin: middleware, эндпоинты API и статика.
// Порядок: Recovery → otelgin → RequestID → RequestLogger (trace_id попадёт в логи).
func NewRouter(h *Handler, staticDir, otelServiceName string) *gin.Engine {
	r := gin.New()                     // Создаём роутер
	r.Use(gin.Recovery())              // Подключаем middleware Recovery
	r.Use(httpx.RequestIDMiddleware()) // Кладём request_id в контекст и заголовок

	// healthcheck/metrics
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API (трейсинг только на этой группе)
	api := r.Group("/",
		otelgin.Middleware(otelServiceName),
		httpx.RequestLogger(h.log),
	)
	api.GET("/order/:id", h.getOrderByID)
	api.GET("/customer/:id/orders", h.listOrdersByCustomer)

	// Static (простая веб-страница)
	if staticDir != "" {
		r.Static("/static", staticDir)
		r.StaticFile("/", filepath.Join(staticDir, "index.html"))
	}

	// 404 для неизвестных маршрутов
	r.NoRoute(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	// 405 для неподдерживаемых методов (разрешаем только GET)
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.Header("Allow", "GET")
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})

	return r
}

// getOrderByID — обработчик GET-запроса /order/:id.
// Возвращает JSON заказа, 404 если не найден, 500 при внутренней ошибке.
// На время обработки ограничиваем контекст таймаутом, чтобы не зависнуть на БД/кэше.
func (h *Handler) getOrderByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.reqTimeout)
	defer cancel()

	order, err := h.service.GetOrder(ctx, id)
	if err != nil {
		h.log.Errorf(ctx, "GetOrder failed id=%s err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

// listOrdersByCustomer — GET /customer/:id/orders?limit=&offset=.
// Возвращает 200 с массивом; 400 — при пустом id; 500 — при ошибке. Пагинация через limit/offset
func (h *Handler) listOrdersByCustomer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty customer id"})
		return
	}

	// Пагинация с безопасными дефолтами и границами
	const (
		defaultLimit = 20
		maxLimit     = 100
	)
	limit, offset := httpx.ParseLimitOffset(c, defaultLimit, maxLimit)

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.reqTimeout)
	defer cancel()

	orders, err := h.service.OrdersByCustomer(ctx, id, limit, offset)
	if err != nil {
		h.log.Errorf(ctx, "OrdersByCustomer failed id=%s err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, orders)
}
