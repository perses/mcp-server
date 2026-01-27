package resource

import "github.com/perses/mcp-server/pkg/tools"

type Resource interface {
	Create() *tools.Tool
	Update() *tools.Tool
	Delete() *tools.Tool
	List() *tools.Tool
	Get() *tools.Tool
	GetTools() []*tools.Tool
}
