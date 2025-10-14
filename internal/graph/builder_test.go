package graph

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalmodfile "github.com/omarshaarawi/gx/internal/modfile"
	"github.com/omarshaarawi/gx/internal/proxy"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

const (
	testGoMod = `module github.com/test/root

go 1.24.2

require (
	github.com/direct/dep1 v1.0.0
	github.com/direct/dep2 v1.1.0
)

require (
	github.com/indirect/dep1 v1.2.0 // indirect
	github.com/indirect/dep2 v1.3.0 // indirect
)
`

	testMinimalGoMod = `module github.com/test/minimal

go 1.24.2
`

	testSingleDepGoMod = `module github.com/test/single

go 1.24.2

require github.com/single/dep v1.0.0
`

	mockDep1GoMod = `module github.com/direct/dep1

go 1.24.2

require github.com/nested/dep v1.0.0
`

	mockDep2GoMod = `module github.com/direct/dep2

go 1.24.2
`

	mockNestedDepGoMod = `module github.com/nested/dep

go 1.24.2
`
)

func TestBuild(t *testing.T) {
	parser := createMockParser(t, testGoMod)

	graph, err := Build(parser)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if graph == nil {
		t.Fatal("Build() returned nil graph")
	}

	if graph.Root == nil {
		t.Fatal("Graph.Root is nil")
	}

	if graph.Root.Path != "github.com/test/root" {
		t.Errorf("Root.Path = %q, want %q", graph.Root.Path, "github.com/test/root")
	}

	if !graph.Root.Direct {
		t.Error("Root.Direct should be true")
	}

	if len(graph.Root.Children) != 2 {
		t.Errorf("Root has %d children, want 2", len(graph.Root.Children))
	}

	if graph.Nodes == nil {
		t.Fatal("Graph.Nodes is nil")
	}
}

func TestBuild_DirectDependencies(t *testing.T) {
	parser := createMockParser(t, testGoMod)

	graph, err := Build(parser)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	expectedDirect := []struct {
		path    string
		version string
	}{
		{"github.com/direct/dep1", "v1.0.0"},
		{"github.com/direct/dep2", "v1.1.0"},
	}

	if len(graph.Root.Children) != len(expectedDirect) {
		t.Fatalf("Root has %d children, want %d", len(graph.Root.Children), len(expectedDirect))
	}

	for i, expected := range expectedDirect {
		child := graph.Root.Children[i]
		if child.Path != expected.path {
			t.Errorf("Child[%d].Path = %q, want %q", i, child.Path, expected.path)
		}
		if child.Version != expected.version {
			t.Errorf("Child[%d].Version = %q, want %q", i, child.Version, expected.version)
		}
		if !child.Direct {
			t.Errorf("Child[%d].Direct = false, want true", i)
		}
	}
}

func TestBuild_IndirectDependencies(t *testing.T) {
	parser := createMockParser(t, testGoMod)

	graph, err := Build(parser)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	indirectDeps := []string{
		"github.com/indirect/dep1",
		"github.com/indirect/dep2",
	}

	for _, path := range indirectDeps {
		node := graph.FindNode(path)
		if node == nil {
			t.Errorf("Indirect dependency %q not found in graph", path)
			continue
		}
		if node.Direct {
			t.Errorf("Node %q should not be direct", path)
		}
	}
}

func TestBuild_MinimalGoMod(t *testing.T) {
	parser := createMockParser(t, testMinimalGoMod)

	graph, err := Build(parser)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(graph.Root.Children) != 0 {
		t.Errorf("Root has %d children, want 0", len(graph.Root.Children))
	}

	if len(graph.Nodes) != 1 {
		t.Errorf("Graph has %d nodes, want 1 (root only)", len(graph.Nodes))
	}
}

func TestBuildWithProxy_NilProxy(t *testing.T) {
	parser := createMockParser(t, testGoMod)

	graph, err := BuildWithProxy(parser, nil)
	if err != nil {
		t.Fatalf("BuildWithProxy(nil) error: %v", err)
	}

	if len(graph.Root.Children) != 2 {
		t.Errorf("Root has %d children, want 2", len(graph.Root.Children))
	}

	for _, child := range graph.Root.Children {
		if len(child.Children) != 0 {
			t.Errorf("Child %q has %d children, want 0 (no proxy traversal)", child.Path, len(child.Children))
		}
	}
}

func TestBuildWithProxy_WithProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case strings.Contains(path, "github.com/direct/dep1") && strings.HasSuffix(path, ".mod"):
			w.Write([]byte(mockDep1GoMod))
		case strings.Contains(path, "github.com/direct/dep2") && strings.HasSuffix(path, ".mod"):
			w.Write([]byte(mockDep2GoMod))
		case strings.Contains(path, "github.com/nested/dep") && strings.HasSuffix(path, ".mod"):
			w.Write([]byte(mockNestedDepGoMod))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := proxy.NewClient(server.URL)
	parser := createMockParser(t, testGoMod)

	graph, err := BuildWithProxy(parser, client)
	if err != nil {
		t.Fatalf("BuildWithProxy() error: %v", err)
	}

	dep1 := findChildByPath(graph.Root.Children, "github.com/direct/dep1")
	if dep1 == nil {
		t.Fatal("dep1 not found in root children")
	}

	if len(dep1.Children) == 0 {
		t.Error("dep1 should have nested children from proxy")
	}

	nestedDep := findChildByPath(dep1.Children, "github.com/nested/dep")
	if nestedDep == nil {
		t.Error("nested/dep not found in dep1 children")
	}
}

func TestBuildWithProxy_MaxDepth(t *testing.T) {
	deepGoMod := `module github.com/deep/dep
go 1.24.2
require github.com/deeper/dep v1.0.0
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(deepGoMod))
	}))
	defer server.Close()

	singleDepParser := createMockParser(t, testSingleDepGoMod)
	client := proxy.NewClient(server.URL)

	graph, err := BuildWithProxy(singleDepParser, client)
	if err != nil {
		t.Fatalf("BuildWithProxy() error: %v", err)
	}

	depth := calculateMaxDepth(graph.Root)
	if depth > 11 {
		t.Errorf("Graph depth = %d, should not exceed 11 (maxDepth=10)", depth)
	}
}

func TestBuildWithProxy_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := proxy.NewClient(server.URL)
	parser := createMockParser(t, testGoMod)

	graph, err := BuildWithProxy(parser, client)
	if err != nil {
		t.Fatalf("BuildWithProxy() should not error on fetch failure: %v", err)
	}

	if graph == nil {
		t.Fatal("BuildWithProxy() returned nil graph")
	}

	if len(graph.Root.Children) != 2 {
		t.Errorf("Root has %d children, want 2", len(graph.Root.Children))
	}

	for _, child := range graph.Root.Children {
		if len(child.Children) != 0 {
			t.Errorf("Child %q has %d children, want 0 (fetch errors)", child.Path, len(child.Children))
		}
	}
}

func TestGraph_GetOrCreateNode(t *testing.T) {
	graph := &Graph{
		Root:  &Node{Path: "root"},
		Nodes: make(map[string]*Node),
	}

	node1 := graph.getOrCreateNode("github.com/test/pkg", "v1.0.0", true)

	if node1 == nil {
		t.Fatal("getOrCreateNode() returned nil")
	}

	if node1.Path != "github.com/test/pkg" {
		t.Errorf("Path = %q, want %q", node1.Path, "github.com/test/pkg")
	}

	if node1.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", node1.Version, "v1.0.0")
	}

	if !node1.Direct {
		t.Error("Direct should be true")
	}

	node2 := graph.getOrCreateNode("github.com/test/pkg", "v1.0.0", false)

	if node2 != node1 {
		t.Error("getOrCreateNode() should return same instance for same path@version")
	}

	if !node2.Direct {
		t.Error("Direct should remain true once set")
	}
}

func TestGraph_GetOrCreateNode_DifferentVersions(t *testing.T) {
	graph := &Graph{
		Root:  &Node{Path: "root"},
		Nodes: make(map[string]*Node),
	}

	node1 := graph.getOrCreateNode("github.com/test/pkg", "v1.0.0", false)
	node2 := graph.getOrCreateNode("github.com/test/pkg", "v2.0.0", false)

	if node1 == node2 {
		t.Error("Different versions should create different nodes")
	}

	if node1.Version == node2.Version {
		t.Error("Versions should differ")
	}
}

func TestGraph_FindNode(t *testing.T) {
	parser := createMockParser(t, testGoMod)
	graph, err := Build(parser)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantFound bool
	}{
		{
			name:      "find root",
			path:      "github.com/test/root",
			wantFound: true,
		},
		{
			name:      "find direct dependency",
			path:      "github.com/direct/dep1",
			wantFound: true,
		},
		{
			name:      "find indirect dependency",
			path:      "github.com/indirect/dep1",
			wantFound: true,
		},
		{
			name:      "not found",
			path:      "github.com/nonexistent/pkg",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := graph.FindNode(tt.path)

			if tt.wantFound {
				if node == nil {
					t.Errorf("FindNode(%q) returned nil, want non-nil", tt.path)
				} else if node.Path != tt.path {
					t.Errorf("FindNode(%q) returned node with path %q", tt.path, node.Path)
				}
			} else {
				if node != nil {
					t.Errorf("FindNode(%q) = %v, want nil", tt.path, node)
				}
			}
		})
	}
}

func TestGraph_FindPaths(t *testing.T) {
	root := &Node{
		Path:     "root",
		Version:  "",
		Direct:   true,
		Children: []*Node{},
	}

	dep1 := &Node{
		Path:     "dep1",
		Version:  "v1.0.0",
		Direct:   true,
		Children: []*Node{},
	}

	nested := &Node{
		Path:     "nested",
		Version:  "v1.0.0",
		Direct:   false,
		Children: []*Node{},
	}

	dep2 := &Node{
		Path:     "dep2",
		Version:  "v1.0.0",
		Direct:   true,
		Children: []*Node{},
	}

	root.Children = []*Node{dep1, dep2}
	dep1.Children = []*Node{nested}

	graph := &Graph{
		Root: root,
		Nodes: map[string]*Node{
			"root":   root,
			"dep1":   dep1,
			"dep2":   dep2,
			"nested": nested,
		},
	}

	tests := []struct {
		name       string
		targetPath string
		wantCount  int
		wantPaths  [][]string
	}{
		{
			name:       "find root",
			targetPath: "root",
			wantCount:  1,
			wantPaths:  [][]string{{"root"}},
		},
		{
			name:       "find direct dependency",
			targetPath: "dep1",
			wantCount:  1,
			wantPaths:  [][]string{{"root", "dep1"}},
		},
		{
			name:       "find nested dependency",
			targetPath: "nested",
			wantCount:  1,
			wantPaths:  [][]string{{"root", "dep1", "nested"}},
		},
		{
			name:       "find non-existent",
			targetPath: "nonexistent",
			wantCount:  0,
			wantPaths:  [][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := graph.FindPaths(tt.targetPath)

			if len(paths) != tt.wantCount {
				t.Errorf("FindPaths(%q) returned %d paths, want %d", tt.targetPath, len(paths), tt.wantCount)
			}

			for i, path := range paths {
				if i >= len(tt.wantPaths) {
					break
				}
				expectedPath := tt.wantPaths[i]
				if len(path) != len(expectedPath) {
					t.Errorf("Path[%d] length = %d, want %d", i, len(path), len(expectedPath))
					continue
				}
				for j, step := range path {
					if step != expectedPath[j] {
						t.Errorf("Path[%d][%d] = %q, want %q", i, j, step, expectedPath[j])
					}
				}
			}
		})
	}
}

func TestGraph_FindPaths_MultiplePaths(t *testing.T) {
	root := &Node{Path: "root", Children: []*Node{}}
	dep1 := &Node{Path: "dep1", Children: []*Node{}}
	dep2 := &Node{Path: "dep2", Children: []*Node{}}
	shared := &Node{Path: "shared", Children: []*Node{}}

	root.Children = []*Node{dep1, dep2}
	dep1.Children = []*Node{shared}
	dep2.Children = []*Node{shared}

	graph := &Graph{
		Root: root,
		Nodes: map[string]*Node{
			"root":   root,
			"dep1":   dep1,
			"dep2":   dep2,
			"shared": shared,
		},
	}

	paths := graph.FindPaths("shared")

	if len(paths) != 2 {
		t.Errorf("FindPaths(shared) returned %d paths, want 2", len(paths))
	}

	hasPath1 := false
	hasPath2 := false

	for _, path := range paths {
		if len(path) != 3 {
			continue
		}
		if path[0] == "root" && path[1] == "dep1" && path[2] == "shared" {
			hasPath1 = true
		}
		if path[0] == "root" && path[1] == "dep2" && path[2] == "shared" {
			hasPath2 = true
		}
	}

	if !hasPath1 {
		t.Error("Missing path: root -> dep1 -> shared")
	}
	if !hasPath2 {
		t.Error("Missing path: root -> dep2 -> shared")
	}
}

func TestBuildFromRequires(t *testing.T) {
	requires := []*modfile.Require{
		{
			Mod: module.Version{
				Path:    "github.com/dep1/pkg",
				Version: "v1.0.0",
			},
			Indirect: false,
		},
		{
			Mod: module.Version{
				Path:    "github.com/dep2/pkg",
				Version: "v2.0.0",
			},
			Indirect: false,
		},
		{
			Mod: module.Version{
				Path:    "github.com/indirect/pkg",
				Version: "v3.0.0",
			},
			Indirect: true,
		},
	}

	graph := BuildFromRequires("github.com/test/module", requires)

	if graph == nil {
		t.Fatal("BuildFromRequires() returned nil")
	}

	if graph.Root.Path != "github.com/test/module" {
		t.Errorf("Root.Path = %q, want %q", graph.Root.Path, "github.com/test/module")
	}

	if len(graph.Root.Children) != 2 {
		t.Errorf("Root has %d children, want 2 (indirect excluded)", len(graph.Root.Children))
	}

	for _, child := range graph.Root.Children {
		if child.Path == "github.com/indirect/pkg" {
			t.Error("Indirect dependency should not be in children")
		}
		if !child.Direct {
			t.Errorf("Child %q should be direct", child.Path)
		}
	}
}

func TestBuildFromRequires_EmptyRequires(t *testing.T) {
	graph := BuildFromRequires("github.com/test/empty", []*modfile.Require{})

	if graph == nil {
		t.Fatal("BuildFromRequires() returned nil")
	}

	if len(graph.Root.Children) != 0 {
		t.Errorf("Root has %d children, want 0", len(graph.Root.Children))
	}

	if len(graph.Nodes) != 1 {
		t.Errorf("Graph has %d nodes, want 1", len(graph.Nodes))
	}
}

func TestBuildFromRequires_OnlyIndirect(t *testing.T) {
	requires := []*modfile.Require{
		{
			Mod: module.Version{
				Path:    "github.com/indirect1/pkg",
				Version: "v1.0.0",
			},
			Indirect: true,
		},
		{
			Mod: module.Version{
				Path:    "github.com/indirect2/pkg",
				Version: "v2.0.0",
			},
			Indirect: true,
		},
	}

	graph := BuildFromRequires("github.com/test/module", requires)

	if len(graph.Root.Children) != 0 {
		t.Errorf("Root has %d children, want 0 (all indirect)", len(graph.Root.Children))
	}
}

func TestNode_Structure(t *testing.T) {
	node := &Node{
		Path:     "github.com/test/pkg",
		Version:  "v1.2.3",
		Direct:   true,
		Children: []*Node{},
	}

	if node.Path != "github.com/test/pkg" {
		t.Errorf("Path = %q, want %q", node.Path, "github.com/test/pkg")
	}

	if node.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", node.Version, "v1.2.3")
	}

	if !node.Direct {
		t.Error("Direct should be true")
	}

	if node.Children == nil {
		t.Error("Children should not be nil")
	}
}

func TestGraph_CircularDependencyPrevention(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "dep1") {
			w.Write([]byte(`module github.com/test/dep1
go 1.24.2
require github.com/test/dep2 v1.0.0
`))
		} else if strings.Contains(r.URL.Path, "dep2") {
			w.Write([]byte(`module github.com/test/dep2
go 1.24.2
require github.com/test/dep1 v1.0.0
`))
		}
	}))
	defer server.Close()

	goMod := `module github.com/test/circular
go 1.24.2
require github.com/test/dep1 v1.0.0
`

	parser := createMockParser(t, goMod)
	client := proxy.NewClient(server.URL)

	graph, err := BuildWithProxy(parser, client)
	if err != nil {
		t.Fatalf("BuildWithProxy() error: %v", err)
	}

	if graph == nil {
		t.Fatal("BuildWithProxy() returned nil")
	}

	depth := calculateMaxDepth(graph.Root)
	if depth > 11 {
		t.Errorf("Circular dependency caused excessive depth: %d", depth)
	}
}

func createMockParser(t testing.TB, content string) *internalmodfile.Parser {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "go.mod")

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp go.mod: %v", err)
	}

	parser, err := internalmodfile.NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	return parser
}

func findChildByPath(children []*Node, path string) *Node {
	for _, child := range children {
		if child.Path == path {
			return child
		}
	}
	return nil
}

func calculateMaxDepth(node *Node) int {
	visited := make(map[string]bool)
	return calculateMaxDepthWithVisited(node, visited)
}

func calculateMaxDepthWithVisited(node *Node, visited map[string]bool) int {
	if node == nil || len(node.Children) == 0 {
		return 0
	}

	nodeKey := node.Path + "@" + node.Version
	if visited[nodeKey] {
		return 0
	}
	visited[nodeKey] = true
	defer func() { visited[nodeKey] = false }()

	maxChildDepth := 0
	for _, child := range node.Children {
		depth := calculateMaxDepthWithVisited(child, visited)
		if depth > maxChildDepth {
			maxChildDepth = depth
		}
	}

	return maxChildDepth + 1
}


func BenchmarkBuild(b *testing.B) {
	parser := createMockParser(b, testGoMod)

	b.ResetTimer()
	for b.Loop(){
		Build(parser)
	}
}

func BenchmarkBuildWithProxy(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mockDep1GoMod))
	}))
	defer server.Close()

	client := proxy.NewClient(server.URL)
	parser := createMockParser(b, testGoMod)

	b.ResetTimer()
	for b.Loop() {
		BuildWithProxy(parser, client)
	}
}

func BenchmarkGraph_FindNode(b *testing.B) {
	parser := createMockParser(b, testGoMod)
	graph, _ := Build(parser)

	b.ResetTimer()
	for b.Loop() {
		graph.FindNode("github.com/direct/dep1")
	}
}

func BenchmarkGraph_FindPaths(b *testing.B) {
	root := &Node{Path: "root", Children: []*Node{}}
	dep1 := &Node{Path: "dep1", Children: []*Node{}}
	dep2 := &Node{Path: "dep2", Children: []*Node{}}
	nested := &Node{Path: "nested", Children: []*Node{}}

	root.Children = []*Node{dep1, dep2}
	dep1.Children = []*Node{nested}

	graph := &Graph{
		Root:  root,
		Nodes: map[string]*Node{"root": root, "dep1": dep1, "dep2": dep2, "nested": nested},
	}

	b.ResetTimer()
	for b.Loop() {
		graph.FindPaths("nested")
	}
}

func BenchmarkBuildFromRequires(b *testing.B) {
	requires := []*modfile.Require{
		{Mod: module.Version{Path: "github.com/dep1/pkg", Version: "v1.0.0"}, Indirect: false},
		{Mod: module.Version{Path: "github.com/dep2/pkg", Version: "v2.0.0"}, Indirect: false},
		{Mod: module.Version{Path: "github.com/dep3/pkg", Version: "v3.0.0"}, Indirect: true},
	}

	b.ResetTimer()
	for b.Loop() {
		BuildFromRequires("github.com/test/module", requires)
	}
}
