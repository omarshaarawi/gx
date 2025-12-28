package outdated

import (
	"context"
	"strings"
	"sync"

	"github.com/omarshaarawi/gx/internal/proxy"
	"github.com/omarshaarawi/gx/internal/ui"
	xmodfile "golang.org/x/mod/modfile"
)

func fetchPackagesWithSpinner(ctx context.Context, proxyClient *proxy.Client, requires []*xmodfile.Require, opts Options) ([]Package, error) {
	return ui.RunWithSpinner(ui.SpinnerTask[[]Package]{
		Message: "Checking for updates...",
		Total:   len(requires),
		Run: func(progress chan<- int) ([]Package, error) {
			return fetchPackages(ctx, proxyClient, requires, opts, progress)
		},
	})
}

func fetchPackages(ctx context.Context, proxyClient *proxy.Client, requires []*xmodfile.Require, opts Options, progressCh chan<- int) ([]Package, error) {
	packages := []Package{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	checked := 0

	for _, req := range requires {
		wg.Add(1)
		go func(r *xmodfile.Require) {
			defer wg.Done()

			latest, err := proxyClient.Latest(ctx, r.Mod.Path)
			if err != nil {
				mu.Lock()
				checked++
				progressCh <- checked
				mu.Unlock()
				return
			}

			updateType := classifyUpdate(r.Mod.Version, latest.Version)

			if opts.MajorOnly && updateType != "major" {
				mu.Lock()
				checked++
				progressCh <- checked
				mu.Unlock()
				return
			}

			pkg := Package{
				Name:       r.Mod.Path,
				Current:    strings.TrimPrefix(r.Mod.Version, "v"),
				Latest:     strings.TrimPrefix(latest.Version, "v"),
				UpdateType: updateType,
				Direct:     !r.Indirect,
			}

			if updateType != "none" {
				mu.Lock()
				packages = append(packages, pkg)
				mu.Unlock()
			}

			mu.Lock()
			checked++
			progressCh <- checked
			mu.Unlock()
		}(req)
	}

	wg.Wait()
	return packages, nil
}
