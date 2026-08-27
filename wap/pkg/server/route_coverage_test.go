package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// Routes that are genuinely safe to reach with a cross-site GET: they read state and change nothing, so a
// page that triggers one learns nothing (it cannot read the response without an allow-listed Origin) and
// breaks nothing. Everything else that does not reject non-POST requests must be in mutatingGETPaths.
//
// Adding a route here is a security decision — it means "a cross-site <img>/<script> may trigger this".
var readOnlyGETRoutes = map[string]string{
	"/readiness":    "reports readiness; no side effects",
	"/properties":   "reads box properties; no side effects",
	"/wifi/list":    "lists visible networks; no side effects",
	"/wifi/status":  "reads connection status; no side effects",
	"/chain/status": "reads chain sync status; no side effects",
	"/account/id":   "reads the account id; no side effects",
	"/account/seed": "reads the seed, but the response is unreadable cross-origin without an allow-listed Origin",
}

// This test exists because the Origin guard silently had a hole: /pools/join, /pools/leave and /pools/cancel
// enforce no HTTP method and read their parameters with r.FormValue, so `GET /pools/join?poolID=…` mutated
// the box config. They were not in mutatingGETPaths, so the guard classified them as non-mutating and let
// them through — an <img src> on any page the owner visited was enough.
//
// Rather than re-listing routes by hand (which is what went wrong), this parses server.go, finds every route
// registered on the mux, and works out whether its handler actually rejects non-POST requests. Any route that
// does NOT must be covered by mutatingGETPaths or explicitly declared read-only above. A new route added
// without a method check therefore fails this test instead of quietly reopening the hole.
func TestEverySideEffectingRouteIsGuarded(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatalf("parse server.go: %v", err)
	}

	handlerEnforcesMethod := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		handlerEnforcesMethod[fn.Name.Name] = bodyChecksMethod(fn.Body)
	}

	routes := registeredRoutes(file)
	if len(routes) == 0 {
		t.Fatal("found no mux.HandleFunc registrations — has the routing style changed? This guard must be updated.")
	}

	for path, h := range routes {
		if h.enforcesMethod(handlerEnforcesMethod) {
			continue // rejects the wrong verb itself, so the method check is the guard
		}
		if mutatingGETPaths[path] {
			continue // guarded by Sec-Fetch-Site / Origin in withCORS
		}
		if _, declared := readOnlyGETRoutes[path]; declared {
			continue // explicitly reviewed as safe to trigger cross-site
		}
		t.Errorf(
			"route %q (handler %s) does not enforce an HTTP method and is neither in mutatingGETPaths nor "+
				"declared read-only.\nIf it changes state, add it to mutatingGETPaths in cors.go — otherwise a "+
				"cross-site <img src=\"http://10.42.0.1:3500%s?...\"> can trigger it.\nIf it is genuinely "+
				"read-only, add it to readOnlyGETRoutes with a reason.",
			path, h.describe(), path,
		)
	}
}

// Everything listed as a mutating GET must actually be a registered route — otherwise the entry is dead and
// gives false confidence that something is protected.
func TestMutatingGETPathsAreRealRoutes(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatalf("parse server.go: %v", err)
	}
	routes := registeredRoutes(file)
	for path := range mutatingGETPaths {
		if _, ok := routes[path]; !ok {
			t.Errorf("mutatingGETPaths lists %q, which is not registered on the mux — stale entry", path)
		}
	}
}

// routeHandler is however a route was registered: a named function, or an inline literal (which in this file
// wraps a named handler to pass it an extra channel — `/wifi/connect` does exactly that).
type routeHandler struct {
	name string
	lit  *ast.FuncLit
}

func (h routeHandler) describe() string {
	if h.name != "" {
		return h.name
	}
	return "inline func literal"
}

// enforcesMethod resolves through an inline literal: the literal itself may check r.Method, or it may simply
// delegate to a named handler that does. Missing this delegation is why the guard first flagged
// /wifi/connect, whose wrapper calls connectWifiHandler — which does enforce POST.
func (h routeHandler) enforcesMethod(named map[string]bool) bool {
	if h.name != "" {
		return named[h.name]
	}
	if h.lit == nil {
		return false
	}
	if bodyChecksMethod(h.lit.Body) {
		return true
	}
	delegates := false
	ast.Inspect(h.lit.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && named[ident.Name] {
			delegates = true
			return false
		}
		return true
	})
	return delegates
}

// registeredRoutes returns path -> handler for every mux.HandleFunc("/path", handler) call in the file.
func registeredRoutes(file *ast.File) map[string]routeHandler {
	routes := map[string]routeHandler{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandleFunc" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		path, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.HasPrefix(path, "/") {
			return true
		}
		var h routeHandler
		switch arg := call.Args[1].(type) {
		case *ast.Ident:
			h.name = arg.Name
		case *ast.FuncLit:
			h.lit = arg
		}
		routes[path] = h
		return true
	})
	return routes
}

// bodyChecksMethod reports whether a handler compares r.Method against something — the pattern the existing
// handlers use to reject anything but their intended verb.
func bodyChecksMethod(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || (bin.Op != token.NEQ && bin.Op != token.EQL) {
			return true
		}
		for _, side := range []ast.Expr{bin.X, bin.Y} {
			if sel, ok := side.(*ast.SelectorExpr); ok && sel.Sel.Name == "Method" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
