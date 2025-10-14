package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	TreeBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	TreeNodeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	TreeVersionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	TreeIndirectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// TreeNode represents a node in a tree structure
type TreeNode struct {
	Label    string
	Version  string
	Indirect bool
	Children []*TreeNode
}

// TreeOptions configures tree rendering
type TreeOptions struct {
	MaxDepth     int
	ShowVersions bool
	Prune        bool // Prune duplicate subtrees
	Pattern      string // Filter pattern
}

// RenderTree renders a tree structure as ASCII art
func RenderTree(root *TreeNode, opts TreeOptions) string {
	if root == nil {
		return ""
	}

	var b strings.Builder
	seen := make(map[string]bool)

	renderNode(&b, root, "", true, 0, opts, seen)
	return b.String()
}

func renderNode(b *strings.Builder, node *TreeNode, prefix string, isLast bool, depth int, opts TreeOptions, seen map[string]bool) {
	if node == nil {
		return
	}

	if opts.MaxDepth > 0 && depth >= opts.MaxDepth {
		return
	}

	if opts.Pattern != "" && !strings.Contains(node.Label, opts.Pattern) {
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			renderNode(b, child, prefix, childIsLast, depth, opts, seen)
		}
		return
	}

	if depth > 0 {
		branch := "├── "
		if isLast {
			branch = "└── "
		}
		b.WriteString(TreeBranchStyle.Render(prefix + branch))
	}

	label := node.Label
	if node.Indirect {
		b.WriteString(TreeIndirectStyle.Render(label))
	} else {
		b.WriteString(TreeNodeStyle.Render(label))
	}

	if opts.ShowVersions && node.Version != "" {
		b.WriteString(TreeVersionStyle.Render("@" + strings.TrimPrefix(node.Version, "v")))
	}

	b.WriteString("\n")

	if opts.Prune {
		nodeKey := node.Label + "@" + node.Version
		if seen[nodeKey] {
			newPrefix := prefix
			if depth > 0 {
				if isLast {
					newPrefix += "    "
				} else {
					newPrefix += "│   "
				}
			}
			b.WriteString(TreeBranchStyle.Render(newPrefix + "└── "))
			b.WriteString(TreeIndirectStyle.Render("(already shown above)"))
			b.WriteString("\n")
			return
		}
		seen[nodeKey] = true
	}

	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		newPrefix := prefix
		if depth > 0 {
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
		}
		renderNode(b, child, newPrefix, childIsLast, depth+1, opts, seen)
	}
}

// SimpleTree creates a simple tree with default options
func SimpleTree(root *TreeNode) string {
	return RenderTree(root, TreeOptions{
		ShowVersions: true,
		Prune:        true,
	})
}

// CompactTree creates a compact tree without versions
func CompactTree(root *TreeNode) string {
	return RenderTree(root, TreeOptions{
		ShowVersions: false,
		Prune:        true,
	})
}

// FullTree creates a full tree without pruning
func FullTree(root *TreeNode) string {
	return RenderTree(root, TreeOptions{
		ShowVersions: true,
		Prune:        false,
	})
}

// CountNodes counts total nodes in a tree
func CountNodes(root *TreeNode) int {
	if root == nil {
		return 0
	}

	count := 1
	for _, child := range root.Children {
		count += CountNodes(child)
	}
	return count
}

// MaxDepth calculates the maximum depth of a tree
func MaxDepth(root *TreeNode) int {
	if root == nil {
		return 0
	}

	if len(root.Children) == 0 {
		return 1
	}

	maxChildDepth := 0
	for _, child := range root.Children {
		childDepth := MaxDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return 1 + maxChildDepth
}
