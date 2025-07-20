package mcpcommon

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
)

func NotifyProgress(ctx context.Context, step int, totalSteps int, message string) {
	s := server.ServerFromContext(ctx)
	req := callToolRequestFromContext(ctx)
	progressToken := req.Params.Meta.ProgressToken
	if progressToken == nil {
		slog.DebugContext(ctx, "no progress token")
		return
	}
	err := s.SendNotificationToClient(ctx, "notification/progress", map[string]any{
		"progress":      step,
		"total":         totalSteps,
		"message":       message,
		"progressToken": progressToken,
	})

	if err != nil {
		slog.ErrorContext(ctx, "error sending progress", "err", err)
	}

	slog.DebugContext(ctx, "sent progress")
}

type ctxKey string

var callToolRequestContextKey = ctxKey("callToolRequest")

func callToolRequestFromContext(ctx context.Context) *mcp.CallToolRequest {
	return ctx.Value(callToolRequestContextKey).(*mcp.CallToolRequest)
}

func withCallToolRequest(ctx context.Context, ctr *mcp.CallToolRequest) context.Context {
	return context.WithValue(ctx, callToolRequestContextKey, ctr)
}
