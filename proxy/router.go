package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Route struct {
	Hostname string `json:"hostname"`
	Backend  string `json:"backend"`
	PID      int    `json:"pid"`
}

type RouteTable struct {
	routes   map[string]Route
	mu       sync.RWMutex
	filePath string
	lockPath string
}

// NewRouteTable builds a route table; lockPath defaults to routes.lock beside filePath.
func NewRouteTable(filePath string) *RouteTable {
	return &RouteTable{
		routes:   make(map[string]Route),
		filePath: filePath,
		lockPath: filepath.Join(filepath.Dir(filePath), "routes.lock"),
	}
}

// acquireDirLock creates lockPath with os.Mkdir (same idea as upstream routes.ts).
// If the directory already exists, it retries with a short sleep; if the lock looks
// stale (mtime older than ~10s), it removes and retries — see phase-3 Task 3.1a.
func (rt *RouteTable) acquireDirLock() error {
	// lockPath lives next to filePath; both need their parent directory to exist.
	if err := os.MkdirAll(filepath.Dir(rt.filePath), 0755); err != nil {
		return fmt.Errorf("ensure parent dir for routes file: %w", err)
	}

	const staleAfter = 10 * time.Second
	const sleep = 50 * time.Millisecond
	deadline := time.Now().Add(30 * time.Second)

	for {
		err := os.Mkdir(rt.lockPath, 0755)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if fi, statErr := os.Stat(rt.lockPath); statErr == nil {
			if time.Since(fi.ModTime()) > staleAfter {
				_ = os.Remove(rt.lockPath)
				continue
			}
		}
		time.Sleep(sleep)
		if time.Now().After(deadline) {
			return fmt.Errorf("acquire directory lock %q: timeout after contention", rt.lockPath)
		}
	}
}

func (rt *RouteTable) releaseDirLock() error {
	return os.Remove(rt.lockPath)
}

// releaseDirLockJoin merges releaseDirLock errors into err so callers do not
// silently drop a failed os.Remove (e.g. permission denied leaving the lock held).
func (rt *RouteTable) releaseDirLockJoin(err error) error {
	if relErr := rt.releaseDirLock(); relErr != nil {
		wrapped := fmt.Errorf("release directory lock: %w", relErr)
		if err == nil {
			return wrapped
		}
		return errors.Join(err, wrapped)
	}
	return err
}

func (rt *RouteTable) AddRoute(hostname, backendURL string) (err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if err = rt.acquireDirLock(); err != nil {
		return err
	}
	defer func() { err = rt.releaseDirLockJoin(err) }()

	rt.routes[hostname] = Route{
		Hostname: hostname,
		Backend:  backendURL,
		PID:      os.Getpid(),
	}
	return rt.save()
}

func (rt *RouteTable) RemoveRoute(hostname string) (err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if err = rt.acquireDirLock(); err != nil {
		return err
	}
	defer func() { err = rt.releaseDirLockJoin(err) }()

	delete(rt.routes, hostname)

	return rt.save()
}

func (rt *RouteTable) Lookup(hostname string) (string, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	v, ok := rt.routes[hostname]
	return v.Backend, ok
}

func (rt *RouteTable) Load() (err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if err = rt.acquireDirLock(); err != nil {
		return err
	}
	defer func() { err = rt.releaseDirLockJoin(err) }()

	data, rerr := os.ReadFile(rt.filePath)
	if rerr != nil {
		if errors.Is(rerr, os.ErrNotExist) {
			rt.routes = make(map[string]Route)
			return nil
		}
		return rerr
	}

	var routes []Route
	if len(data) == 0 {
		rt.routes = make(map[string]Route)
		return nil
	}

	if err := json.Unmarshal(data, &routes); err != nil {
		return err
	}

	rt.routes = make(map[string]Route, len(routes))
	pruned := false
	for _, route := range routes {
		if routeProcessAlive(route.PID) {
			rt.routes[route.Hostname] = route
		} else {
			pruned = true
		}
	}

	if pruned {
		return rt.save()
	}

	return nil
}

func (rt *RouteTable) save() error {
	routes := make([]Route, 0, len(rt.routes))
	for hostname, route := range rt.routes {
		routes = append(routes, Route{
			Hostname: hostname,
			Backend:  route.Backend,
			PID:      route.PID,
		})
	}
	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(rt.filePath, data, 0644)
}

// routeProcessAlive reports whether this route's owning PID still exists.
// PID <= 0 means "no owner" (missing or zero pid in JSON) — we treat that like
// a dead owner and drop the route on Load so legacy files cannot keep stale entries.
func routeProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
