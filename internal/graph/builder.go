package graph

import (
	"context"

	"github.com/omarshaarawi/gx/internal/modfile"
	"github.com/omarshaarawi/gx/internal/proxy"
	xmodfile "golang.org/x/mod/modfile"
)

// Node represents a module in the dependency graph
type Node struct {
	Path     string
	Version  string
	Direct   bool
	Children []*Node
}

// Graph represents a module dependency graph
type Graph struct {
	Root  *Node
	Nodes map[string]*Node
}

// Build builds a dependency graph from a go.mod file
func Build(parser *modfile.Parser) (*Graph, error) {
	return BuildWithProxy(parser, nil)
}

// BuildWithProxy builds a dependency graph, optionally fetching dependencies from proxy
func BuildWithProxy(parser *modfile.Parser, proxyClient *proxy.Client) (*Graph, error) {
	root := &Node{
		Path:     parser.ModulePath(),
		Version:  "",
		Direct:   true,
		Children: []*Node{},
	}

	graph := &Graph{
		Root:  root,
		Nodes: make(map[string]*Node),
	}

	graph.Nodes[root.Path] = root

	if proxyClient == nil {
		for _, req := range parser.DirectRequires() {
			child := graph.getOrCreateNode(req.Mod.Path, req.Mod.Version, true)
			root.Children = append(root.Children, child)
		}

		for _, req := range parser.IndirectRequires() {
			graph.getOrCreateNode(req.Mod.Path, req.Mod.Version, false)
		}

		return graph, nil
	}

	ctx := context.Background()
	visited := make(map[string]bool)

	for _, req := range parser.DirectRequires() {
		child := graph.getOrCreateNode(req.Mod.Path, req.Mod.Version, true)
		root.Children = append(root.Children, child)

		graph.buildChildren(ctx, proxyClient, child, visited, 0, 10)
	}

	return graph, nil
}

// buildChildren recursively builds the dependency tree
func (g *Graph) buildChildren(ctx context.Context, client *proxy.Client, node *Node, visited map[string]bool, depth, maxDepth int) {
	if depth >= maxDepth {
		return
	}

	nodeKey := node.Path + "@" + node.Version
	if visited[nodeKey] {
		return
	}
	visited[nodeKey] = true

	modData, err := client.GetModFile(ctx, node.Path, node.Version)
	if err != nil {
		return
	}

	modFile, err := xmodfile.Parse("go.mod", modData, nil)
	if err != nil {
		return
	}

	for _, req := range modFile.Require {
		if req.Indirect {
			continue
		}

		child := g.getOrCreateNode(req.Mod.Path, req.Mod.Version, false)

		alreadyChild := false
		for _, existing := range node.Children {
			if existing.Path == child.Path {
				alreadyChild = true
				break
			}
		}

		if !alreadyChild {
			node.Children = append(node.Children, child)

			g.buildChildren(ctx, client, child, visited, depth+1, maxDepth)
		}
	}
}

// getOrCreateNode gets or creates a node in the graph
func (g *Graph) getOrCreateNode(path, version string, direct bool) *Node {
	nodeKey := path + "@" + version

	if node, exists := g.Nodes[nodeKey]; exists {
		if direct {
			node.Direct = true
		}
		return node
	}

	node := &Node{
		Path:     path,
		Version:  version,
		Direct:   direct,
		Children: []*Node{},
	}

	g.Nodes[nodeKey] = node
	g.Nodes[path] = node
	return node
}

// FindNode finds a node by path
func (g *Graph) FindNode(path string) *Node {
	return g.Nodes[path]
}

// FindPaths finds all paths from root to target
func (g *Graph) FindPaths(targetPath string) [][]string {
	var paths [][]string
	var currentPath []string

	visited := make(map[string]bool)

	var dfs func(node *Node)
	dfs = func(node *Node) {
		if visited[node.Path] {
			return
		}

		currentPath = append(currentPath, node.Path)
		visited[node.Path] = true

		if node.Path == targetPath {
			pathCopy := make([]string, len(currentPath))
			copy(pathCopy, currentPath)
			paths = append(paths, pathCopy)
		} else {
			for _, child := range node.Children {
				dfs(child)
			}
		}

		currentPath = currentPath[:len(currentPath)-1]
		visited[node.Path] = false
	}

	dfs(g.Root)
	return paths
}

// BuildFromRequires builds a simple graph structure from requires
func BuildFromRequires(modulePath string, requires []*xmodfile.Require) *Graph {
	root := &Node{
		Path:     modulePath,
		Version:  "",
		Direct:   true,
		Children: []*Node{},
	}

	graph := &Graph{
		Root:  root,
		Nodes: make(map[string]*Node),
	}

	graph.Nodes[root.Path] = root

	for _, req := range requires {
		if req.Indirect {
			continue
		}

		child := &Node{
			Path:     req.Mod.Path,
			Version:  req.Mod.Version,
			Direct:   !req.Indirect,
			Children: []*Node{},
		}

		root.Children = append(root.Children, child)
		graph.Nodes[child.Path] = child
	}

	return graph
}

