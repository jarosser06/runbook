package server

import (
	"context"
	"fmt"

	"github.com/jarosser06/dev-workflow-mcp/internal/template"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerPrompts registers all prompts as MCP prompts
func (s *Server) registerPrompts() {
	for promptName, promptDef := range s.manifest.Prompts {
		// Capture variables for closure
		name := promptName
		def := promptDef

		prompt := mcp.Prompt{
			Name:        name,
			Description: def.Description,
		}

		handler := func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			// Resolve template variables in prompt content
			resolvedContent, err := template.ResolvePromptTemplate(def.Content, s.manifest.Tasks)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve prompt template: %w", err)
			}

			return &mcp.GetPromptResult{
				Description: def.Description,
				Messages: []mcp.PromptMessage{
					{
						Role: mcp.RoleUser,
						Content: mcp.TextContent{
							Type: "text",
							Text: resolvedContent,
						},
					},
				},
			}, nil
		}

		s.mcpServer.AddPrompt(prompt, handler)
	}
}
