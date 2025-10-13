package modfile

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"
)

// Parser wraps golang modfile with additional utilities
type Parser struct {
	path string
	file *modfile.File
	data []byte
}

// NewParser creates a new modfile parser
func NewParser(path string) (*Parser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return &Parser{
		path: path,
		file: file,
		data: data,
	}, nil
}

// File returns the underlying modfile.File
func (p *Parser) File() *modfile.File {
	return p.file
}

// ModulePath returns the module path
func (p *Parser) ModulePath() string {
	if p.file.Module == nil {
		return ""
	}

	return p.file.Module.Mod.Path
}

// DirectRequires returns all direct (non-indirect) requirements
func (p *Parser) DirectRequires() []*modfile.Require {
	var direct []*modfile.Require
	for _, req := range p.file.Require {
		if !req.Indirect {
			direct = append(direct, req)
		}
	}
	return direct
}

// IndirectRequires returns all indirect requirements
func (p *Parser) IndirectRequires() []*modfile.Require {
	var indirect []*modfile.Require
	for _, req := range p.file.Require {
		if req.Indirect {
			indirect = append(indirect, req)
		}
	}
	return indirect
}

// AllRequires returns all requirements
func (p *Parser) AllRequires() []*modfile.Require {
	return p.file.Require
}

// FindRequire finds a requirement by module path
func (p *Parser) FindRequire(modulePath string) *modfile.Require {
	for _, req := range p.file.Require {
		if req.Mod.Path == modulePath {
			return req
		}
	}
	return nil
}

// HasRequire checks if a module is required
func (p *Parser) HasRequire(modulePath string) bool {
	return p.FindRequire(modulePath) != nil
}


