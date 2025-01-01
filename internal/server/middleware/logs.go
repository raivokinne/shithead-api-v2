package middleware

import (
    "context"
    "log/slog"
    "os"
    "path/filepath"
    "time"

    "github.com/gofiber/fiber/v2"
)

type LogMiddleware struct {
    logger *slog.Logger
}

func NewLogMiddleware() (*LogMiddleware, error) {
    if err := os.MkdirAll("logs", 0755); err != nil {
        return nil, err
    }

    logFile, err := os.OpenFile(
        filepath.Join("logs", "api.log"),
        os.O_CREATE|os.O_WRONLY|os.O_APPEND,
        0644,
        )
    if err != nil {
        return nil, err
    }

    opts := &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }
    handler := slog.NewJSONHandler(logFile, opts)
    logger := slog.New(handler)

    return &LogMiddleware{
        logger: logger,
    }, nil
}

func (l *LogMiddleware) Handle(c *fiber.Ctx) error {
    start := time.Now()

    requestID := c.Get("X-Request-ID")
    if requestID == "" {
        requestID = "unknown"
    }

    userID := "anonymous"
    if user := c.Locals("user"); user != nil {
        if u, ok := user.(map[string]interface{}); ok {
            if id, exists := u["id"]; exists {
                userID = id.(string)
            }
        }
    }

    err := c.Next()

    duration := time.Since(start)

    l.logger.LogAttrs(context.Background(),
        slog.LevelInfo,
        "http_request",
        slog.String("request_id", requestID),
        slog.String("user_id", userID),
        slog.String("method", c.Method()),
        slog.String("path", c.Path()),
        slog.String("ip", c.IP()),
        slog.Int("status", c.Response().StatusCode()),
        slog.Duration("duration", duration),
        slog.String("user_agent", c.Get("User-Agent")),
        slog.Int64("bytes_received", int64(len(c.Request().Body()))),
        slog.Int64("bytes_sent", int64(len(c.Response().Body()))),
        )

    return err
}
