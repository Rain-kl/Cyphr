// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"strings"
	"sync"
)

// RouteDefinition holds the metadata and handler list for a single HTTP route.
type RouteDefinition struct {
	ID          uint64
	Method      string
	Path        string
	Handlers    []any
	Middlewares []any
}

// RouterExtension defines the interface for registering routes and middlewares.
type RouterExtension interface {
	Use(middlewares ...any)
	Group(prefix string, middlewares ...any) RouterExtension
	Handle(method, path string, handlers ...any) RouteDefinition
	GET(path string, handlers ...any) RouteDefinition
	POST(path string, handlers ...any) RouteDefinition
	PUT(path string, handlers ...any) RouteDefinition
	DELETE(path string, handlers ...any) RouteDefinition
	PATCH(path string, handlers ...any) RouteDefinition
	HEAD(path string, handlers ...any) RouteDefinition
	OPTIONS(path string, handlers ...any) RouteDefinition
	Any(path string, handlers ...any) []RouteDefinition
	Routes() []RouteDefinition
	Middlewares() []any
	Unregister(method, path string) bool
	UnregisterByID(id uint64) bool
	RegisterWhitelist(patterns ...string)
	Whitelist() []string
	IsWhitelisted(path string) bool
}

// RouterRegistry implements RouterExtension as the root route and middleware collector.
type RouterRegistry struct {
	mu          sync.RWMutex
	nextID      uint64
	routes      []RouteDefinition
	middlewares []any
	whitelist   []string
}

// NewRouterRegistry creates a new root router collector.
func NewRouterRegistry() *RouterRegistry {
	return &RouterRegistry{}
}

// Use registers global middlewares to the router.
func (r *RouterRegistry) Use(middlewares ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, middlewares...)
}

// Middlewares returns a copy of registered root middlewares.
func (r *RouterRegistry) Middlewares() []any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]any, len(r.middlewares))
	copy(res, r.middlewares)
	return res
}

// Group creates a new RouteGroup under the router.
func (r *RouterRegistry) Group(prefix string, middlewares ...any) RouterExtension {
	return &RouterGroup{
		registry:    r,
		prefix:      cleanPath(prefix),
		middlewares: middlewares,
	}
}

// Handle registers a route with a custom HTTP method and handlers.
func (r *RouterRegistry) Handle(method, path string, handlers ...any) RouteDefinition {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	rd := RouteDefinition{
		ID:          r.nextID,
		Method:      strings.ToUpper(method),
		Path:        cleanPath(path),
		Handlers:    handlers,
		Middlewares: append([]any(nil), r.middlewares...),
	}
	r.routes = append(r.routes, rd)
	return rd
}

// Unregister removes a route matching method and path from the registry.
func (r *RouterRegistry) Unregister(method, path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	targetMethod := strings.ToUpper(method)
	targetPath := cleanPath(path)

	for i, rd := range r.routes {
		if rd.Method == targetMethod && rd.Path == targetPath {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// UnregisterByID removes a route by its unique ID.
func (r *RouterRegistry) UnregisterByID(id uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rd := range r.routes {
		if rd.ID == id {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// GET registers a GET route.
func (r *RouterRegistry) GET(path string, handlers ...any) RouteDefinition {
	return r.Handle("GET", path, handlers...)
}

// POST registers a POST route.
func (r *RouterRegistry) POST(path string, handlers ...any) RouteDefinition {
	return r.Handle("POST", path, handlers...)
}

// PUT registers a PUT route.
func (r *RouterRegistry) PUT(path string, handlers ...any) RouteDefinition {
	return r.Handle("PUT", path, handlers...)
}

// DELETE registers a DELETE route.
func (r *RouterRegistry) DELETE(path string, handlers ...any) RouteDefinition {
	return r.Handle("DELETE", path, handlers...)
}

// PATCH registers a PATCH route.
func (r *RouterRegistry) PATCH(path string, handlers ...any) RouteDefinition {
	return r.Handle("PATCH", path, handlers...)
}

// HEAD registers a HEAD route.
func (r *RouterRegistry) HEAD(path string, handlers ...any) RouteDefinition {
	return r.Handle("HEAD", path, handlers...)
}

// OPTIONS registers an OPTIONS route.
func (r *RouterRegistry) OPTIONS(path string, handlers ...any) RouteDefinition {
	return r.Handle("OPTIONS", path, handlers...)
}

// Any registers a route for standard HTTP methods.
func (r *RouterRegistry) Any(path string, handlers ...any) []RouteDefinition {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	defs := make([]RouteDefinition, 0, len(methods))
	for _, m := range methods {
		defs = append(defs, r.Handle(m, path, handlers...))
	}
	return defs
}

// Routes returns a copy of all collected RouteDefinitions.
func (r *RouterRegistry) Routes() []RouteDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]RouteDefinition, len(r.routes))
	copy(res, r.routes)
	return res
}

// RegisterWhitelist adds path patterns to the whitelist.
func (r *RouterRegistry) RegisterWhitelist(patterns ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range patterns {
		clean := cleanPath(p)
		if clean != "" {
			r.whitelist = append(r.whitelist, clean)
		}
	}
}

// Whitelist returns a copy of all registered whitelist path patterns.
func (r *RouterRegistry) Whitelist() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, len(r.whitelist))
	copy(res, r.whitelist)
	return res
}

// IsWhitelisted checks if the given path matches any registered whitelist pattern.
func (r *RouterRegistry) IsWhitelisted(path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clean := cleanPath(path)
	for _, pattern := range r.whitelist {
		if MatchPathPattern(pattern, clean) {
			return true
		}
	}
	return false
}

// RouterGroup represents a scoped route group with a path prefix and group-level middlewares.
type RouterGroup struct {
	registry    *RouterRegistry
	prefix      string
	middlewares []any
}

// Use adds middlewares to this group.
func (g *RouterGroup) Use(middlewares ...any) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// Group creates a nested RouteGroup.
func (g *RouterGroup) Group(prefix string, middlewares ...any) RouterExtension {
	combinedPrefix := joinPaths(g.prefix, prefix)
	combinedMiddlewares := make([]any, 0, len(g.middlewares)+len(middlewares))
	combinedMiddlewares = append(combinedMiddlewares, g.middlewares...)
	combinedMiddlewares = append(combinedMiddlewares, middlewares...)

	return &RouterGroup{
		registry:    g.registry,
		prefix:      combinedPrefix,
		middlewares: combinedMiddlewares,
	}
}

// Handle registers a route under this group.
func (g *RouterGroup) Handle(method, path string, handlers ...any) RouteDefinition {
	g.registry.mu.Lock()
	defer g.registry.mu.Unlock()

	fullPath := joinPaths(g.prefix, path)

	allMiddlewares := make([]any, 0, len(g.registry.middlewares)+len(g.middlewares))
	allMiddlewares = append(allMiddlewares, g.registry.middlewares...)
	allMiddlewares = append(allMiddlewares, g.middlewares...)

	g.registry.nextID++
	rd := RouteDefinition{
		ID:          g.registry.nextID,
		Method:      strings.ToUpper(method),
		Path:        fullPath,
		Handlers:    handlers,
		Middlewares: allMiddlewares,
	}
	g.registry.routes = append(g.registry.routes, rd)
	return rd
}

// Unregister removes a route under this group prefix matching method and path.
func (g *RouterGroup) Unregister(method, path string) bool {
	fullPath := joinPaths(g.prefix, path)
	return g.registry.Unregister(method, fullPath)
}

// UnregisterByID removes a route by its unique ID.
func (g *RouterGroup) UnregisterByID(id uint64) bool {
	return g.registry.UnregisterByID(id)
}

// GET registers a GET route in this group.
func (g *RouterGroup) GET(path string, handlers ...any) RouteDefinition {
	return g.Handle("GET", path, handlers...)
}

// POST registers a POST route in this group.
func (g *RouterGroup) POST(path string, handlers ...any) RouteDefinition {
	return g.Handle("POST", path, handlers...)
}

// PUT registers a PUT route in this group.
func (g *RouterGroup) PUT(path string, handlers ...any) RouteDefinition {
	return g.Handle("PUT", path, handlers...)
}

// DELETE registers a DELETE route in this group.
func (g *RouterGroup) DELETE(path string, handlers ...any) RouteDefinition {
	return g.Handle("DELETE", path, handlers...)
}

// PATCH registers a PATCH route in this group.
func (g *RouterGroup) PATCH(path string, handlers ...any) RouteDefinition {
	return g.Handle("PATCH", path, handlers...)
}

// HEAD registers a HEAD route in this group.
func (g *RouterGroup) HEAD(path string, handlers ...any) RouteDefinition {
	return g.Handle("HEAD", path, handlers...)
}

// OPTIONS registers an OPTIONS route in this group.
func (g *RouterGroup) OPTIONS(path string, handlers ...any) RouteDefinition {
	return g.Handle("OPTIONS", path, handlers...)
}

// Any registers a route in this group for standard HTTP methods.
func (g *RouterGroup) Any(path string, handlers ...any) []RouteDefinition {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	defs := make([]RouteDefinition, 0, len(methods))
	for _, m := range methods {
		defs = append(defs, g.Handle(m, path, handlers...))
	}
	return defs
}

// Routes returns all routes from the parent registry.
func (g *RouterGroup) Routes() []RouteDefinition {
	return g.registry.Routes()
}

// Middlewares returns a copy of the group's middlewares.
func (g *RouterGroup) Middlewares() []any {
	res := make([]any, len(g.middlewares))
	copy(res, g.middlewares)
	return res
}

// RegisterWhitelist adds path patterns under this group prefix to the whitelist.
func (g *RouterGroup) RegisterWhitelist(patterns ...string) {
	for _, p := range patterns {
		g.registry.RegisterWhitelist(joinPaths(g.prefix, p))
	}
}

// Whitelist returns a copy of all registered whitelist path patterns.
func (g *RouterGroup) Whitelist() []string {
	return g.registry.Whitelist()
}

// IsWhitelisted checks if the given path matches any registered whitelist pattern.
func (g *RouterGroup) IsWhitelisted(path string) bool {
	return g.registry.IsWhitelisted(path)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func joinPaths(base, relative string) string {
	if base == "" || base == "/" {
		return cleanPath(relative)
	}
	if relative == "" || relative == "/" {
		return cleanPath(base)
	}
	base = strings.TrimSuffix(base, "/")
	relative = strings.TrimPrefix(relative, "/")
	return cleanPath(base + "/" + relative)
}

// MatchPathPattern checks if a URL path matches a pattern (supports exact match and wildcards).
func MatchPathPattern(pattern, path string) bool {
	pattern = cleanPath(pattern)
	path = cleanPath(path)

	if pattern == path {
		return true
	}

	// Suffix wildcard: /api/v1/oauth/* matches /api/v1/oauth and /api/v1/oauth/...
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	// Parameter wildcard: /api/v1/oauth/*/authorize or /api/v1/oauth/:source/authorize
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	if len(patternParts) == len(pathParts) {
		matched := true
		for i, part := range patternParts {
			if part == "*" || strings.HasPrefix(part, ":") {
				continue
			}
			if part != pathParts[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}
