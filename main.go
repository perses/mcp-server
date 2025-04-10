package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	persesClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

// steps
// 1. initialize Perses client
// 2. get the list of projects
func main() {
	s := server.NewMCPServer(
		"perses-mcp",
		"0.0.1",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	s.AddTool(getDashboards())

	if err := server.ServeStdio(s); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func getDashboards() (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_projects", mcp.WithDescription("Get projects")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// simulate fetching projects
		projects := "project1, project2, project3, project4, project5"
		// convert to comma separated string
		return mcp.NewToolResultText(string(projects)), nil
	}
}

func initializePersesClient(baseURL string) persesClient.ClientInterface {

	restClient, err := config.NewRESTClient(config.RestConfigClient{
		URL: common.MustParseURL(baseURL),
	})
	if err != nil {
		fmt.Println("Error creating REST client:", err)
		return nil
	}

	client := persesClient.NewWithClient(restClient)
	return client
}

// func main() {
// 	scanner := bufio.NewScanner(os.Stdin)
// 	fmt.Println("Enter text (type 'exit' to quit):")

// 	for scanner.Scan() {
// 		text := scanner.Text()
// 		fmt.Println("You entered:", text)
// 		if text == "exit" {
// 			break
// 		}
// 	}
// }
