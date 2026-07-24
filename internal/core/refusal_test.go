package core

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestTypedRefusalShape(t *testing.T) {
	// Every code in the recovery table must produce a fully populated shape:
	// a blank field is exactly the gap that makes an agent improvise (R4.1).
	for code := range refusalRecovery {
		refusal := Refuse(code, "blocked")
		if refusal.Code == "" || refusal.Blocker == "" {
			t.Fatalf("%s: code=%q blocker=%q", code, refusal.Code, refusal.Blocker)
		}
		if refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
			t.Fatalf("%s: actor=%q recovery=%q", code, refusal.ActorRequired, refusal.RecoveryCommand)
		}
		switch refusal.ActorRequired {
		case RefusalActorAgent, RefusalActorHuman, RefusalActorOperator:
		default:
			t.Fatalf("%s: actor class=%q", code, refusal.ActorRequired)
		}
		// A refusal only a human or operator can clear is never retry-safe for
		// the agent that hit it.
		if refusal.ActorRequired != RefusalActorAgent && refusal.RetrySafe {
			t.Fatalf("%s: retry_safe with actor=%s", code, refusal.ActorRequired)
		}
	}
}

func TestTypedRefusalUnknownCodeStillStructured(t *testing.T) {
	refusal := Refuse("NOT_IN_TABLE", "blocked")
	if refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
		t.Fatalf("unknown code left an empty field: %#v", refusal)
	}
	if refusal.Code != "NOT_IN_TABLE" {
		t.Fatalf("code=%q", refusal.Code)
	}
}

// TestRefusalCodesRegistered is the construction-site conformance check: a
// literal constructor code or a code emitted through a TransitionBlocker
// cannot silently inherit the generic fallback.
func TestRefusalCodesRegistered(t *testing.T) {
	walkProductionGo(t, func(path string, fset *token.FileSet, file *ast.File) {
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && untrackedRefusalConstructor(path, file, call) {
				position := fset.Position(call.Pos())
				t.Errorf("%s:%d: nonliteral refusal code is not a TransitionBlocker code", path, position.Line)
				return true
			}
			code, position, ok := refusalCodeLiteral(node, fset)
			if !ok {
				return true
			}
			if _, ok := refusalRecovery[code]; !ok {
				t.Errorf("%s:%d: refusal code %q is absent from refusalRecovery", path, position.Line, code)
			}
			return true
		})
	})
}

func TestRefusalCodesRegisteredRejectsVariableCode(t *testing.T) {
	if !fixtureHasUntrackedRefusal(t, `package fixture
func future(code string) error { return Refuse(code, "blocked") }
`) {
		t.Fatal("future variable refusal code was not rejected")
	}
}

func TestRefusalCodesRegisteredSelectorProvenance(t *testing.T) {
	rejected := []string{
		`package fixture
type Request struct { Code string }
func future(request Request) error { return Refuse(request.Code, "blocked") }
`,
		`package fixture
type Config struct { Code string }
func future(config Config) error { return Refuse(config.Code, "blocked") }
`,
	}
	for _, source := range rejected {
		if !fixtureHasUntrackedRefusal(t, source) {
			t.Fatal("unproved selector refusal code was not rejected")
		}
	}
	if fixtureHasUntrackedRefusal(t, `package fixture
type TransitionBlocker struct { Code string }
func future(blocker TransitionBlocker) error { return Refuse(blocker.Code, "blocked") }
`) {
		t.Fatal("TransitionBlocker selector refusal code was rejected")
	}
}

func TestTypedRefusalBeforeAuthorityReportsNotConsumed(t *testing.T) {
	// A refusal raised before authority is issued consumed nothing, so a retry
	// does not need a fresh packet.
	refusal := Refuse("PHASE_INVALID", "phase is perceive")
	if refusal.AuthorityConsumed {
		t.Fatal("refusal before authority issue reports authority_consumed true")
	}
	if !refusal.RetrySafe {
		t.Fatal("agent-clearable refusal is not retry safe")
	}

	consumed := refusal.Consumed()
	if !consumed.AuthorityConsumed || consumed.RetrySafe {
		t.Fatalf("Consumed() = %#v", consumed)
	}
	// Consumed returns a copy; the original must not change underneath a caller.
	if refusal.AuthorityConsumed {
		t.Fatal("Consumed mutated the receiver")
	}
}

func TestTypedRefusalHumanOnlyIsNotAgentRetryable(t *testing.T) {
	refusal := Refuse("APPROVAL_REQUIRED", "gate design awaits approval")
	if refusal.ActorRequired != RefusalActorHuman {
		t.Fatalf("actor=%q", refusal.ActorRequired)
	}
	if refusal.RetrySafe {
		t.Fatal("approval refusal advertised as agent-retryable")
	}
}

func TestTypedRefusalWrappingKeepsSentinel(t *testing.T) {
	sentinel := errors.New("unknown command")
	err := error(Refuse("UNKNOWN_COMMAND", "unknown command \"nope\"").Wrapping(sentinel))
	if !errors.Is(err, sentinel) {
		t.Fatal("wrapped refusal lost its sentinel")
	}
	refusal, ok := AsRefusal(err)
	if !ok {
		t.Fatal("AsRefusal did not recover the shape")
	}
	if refusal.Code != "UNKNOWN_COMMAND" {
		t.Fatalf("code=%q", refusal.Code)
	}
}

func TestTypedRefusalSerializesEveryField(t *testing.T) {
	raw, err := json.Marshal(Refuse("EVIDENCE_MISSING", "no passing verify record"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	// R4.2: one shape on every machine refusal path, so every field is always
	// present — an absent key is indistinguishable from a false value.
	for _, field := range []string{"code", "category", "entity", "observed", "expected", "input_digests", "state_changed", "checkpoint_id", "retryable", "actor_required", "recovery_operations", "detail", "blocker", "authority_consumed", "retry_safe", "recovery_command"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("refusal JSON omits %q: %s", field, raw)
		}
	}
}

func TestRefusalRecoveryContract(t *testing.T) {
	secret := "Bearer do-not-leak"
	r := Refuse("EVIDENCE_FAILING", "verify exited 1").
		WithContext("demo/T1", "exit_code=1", "exit_code=0 at current HEAD").
		WithInput("evidence.jsonl", []byte(secret)).
		WithMutation(true, "evidence.jsonl#T1")
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); len(r.InputDigests["evidence.jsonl"]) != 64 || strings.Contains(got, secret) {
		t.Fatalf("refusal leaked input instead of digest: %s", got)
	}
	if !r.StateChanged || r.CheckpointID == "" || !r.Retryable || len(r.RecoveryOperations) != 1 || r.RecoveryOperations[0].Operation != "verify.task" {
		t.Fatalf("incomplete recovery contract: %#v", r)
	}

	terminal := Refuse("NO_SUCCESSOR", "released history is immutable").
		WithSuccessor(RefusalActorHuman, "new", "specd new <successor>")
	if terminal.Retryable || terminal.RecoveryOperations[0].InPlace || terminal.RecoveryOperations[0].Operation != "new" {
		t.Fatalf("terminal refusal advertises in-place retry: %#v", terminal)
	}
}

// TestWorkerOutOfScopeRefusalClass pins spec R6.4: WORKER_OUT_OF_SCOPE is a
// non-retryable, operator-cleared scope-class refusal — a class refusal, not a
// retryable warning.
func TestWorkerOutOfScopeRefusalClass(t *testing.T) {
	r := Refusef("WORKER_OUT_OF_SCOPE", "mission m1 (task T1) pinned to %q claimed by %q", "w1", "w2")
	if r.Category != "scope" {
		t.Fatalf("category = %q, want scope", r.Category)
	}
	if r.Retryable || r.RetrySafe {
		t.Fatal("out-of-scope refusal must not be retryable")
	}
	if r.ActorRequired != RefusalActorOperator {
		t.Fatalf("actor = %q, want operator", r.ActorRequired)
	}
	if _, ok := AsRefusal(error(r)); !ok {
		t.Fatal("WORKER_OUT_OF_SCOPE not recognized as a typed refusal")
	}
}

func isRefusalConstructor(expr ast.Expr) bool {
	switch fn := expr.(type) {
	case *ast.Ident:
		return fn.Name == "Refuse" || fn.Name == "Refusef"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "Refuse" || fn.Sel.Name == "Refusef"
	default:
		return false
	}
}

func untrackedRefusalConstructor(path string, file *ast.File, call *ast.CallExpr) bool {
	if !isRefusalConstructor(call.Fun) || len(call.Args) == 0 {
		return isRefusalConstructor(call.Fun)
	}
	if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
		return false
	}
	if transitionBlockerCode(file, call, call.Args[0]) {
		return false
	}
	if path == "internal/core/refusal.go" {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.Name == "Refusef" && function.Body.Pos() <= call.Pos() && call.Pos() <= function.Body.End() {
				return false
			}
		}
	}
	return true
}

func transitionBlockerCode(file *ast.File, call *ast.CallExpr, expression ast.Expr) bool {
	code, ok := expression.(*ast.SelectorExpr)
	if !ok || code.Sel.Name != "Code" {
		return false
	}
	if blockerIndex(code.X) {
		return true
	}
	identifier, ok := code.X.(*ast.Ident)
	if !ok {
		return false
	}
	function := enclosingFunction(file, call.Pos())
	if function == nil {
		return false
	}
	for _, field := range function.Type.Params.List {
		if transitionBlockerType(field.Type) {
			for _, name := range field.Names {
				if name.Name == identifier.Name {
					return true
				}
			}
		}
	}
	proven := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if node == nil || proven || node.Pos() >= call.Pos() {
			return false
		}
		switch statement := node.(type) {
		case *ast.AssignStmt:
			for index, target := range statement.Lhs {
				name, ok := target.(*ast.Ident)
				if ok && name.Name == identifier.Name && index < len(statement.Rhs) && blockerIndex(statement.Rhs[index]) {
					proven = true
				}
			}
		case *ast.DeclStmt:
			declaration, ok := statement.Decl.(*ast.GenDecl)
			if !ok {
				break
			}
			for _, spec := range declaration.Specs {
				variable, ok := spec.(*ast.ValueSpec)
				if !ok || !transitionBlockerType(variable.Type) {
					continue
				}
				for _, name := range variable.Names {
					proven = proven || name.Name == identifier.Name
				}
			}
		case *ast.RangeStmt:
			name, ok := statement.Value.(*ast.Ident)
			proven = ok && name.Name == identifier.Name && blockerCollection(statement.X)
		}
		return !proven
	})
	return proven
}

func blockerIndex(expression ast.Expr) bool {
	index, ok := expression.(*ast.IndexExpr)
	return ok && blockerCollection(index.X)
}

func blockerCollection(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Blockers"
}

func transitionBlockerType(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "TransitionBlocker"
	case *ast.SelectorExpr:
		return value.Sel.Name == "TransitionBlocker"
	case *ast.StarExpr:
		return transitionBlockerType(value.X)
	default:
		return false
	}
}

func enclosingFunction(file *ast.File, position token.Pos) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Body != nil && function.Body.Pos() <= position && position <= function.Body.End() {
			return function
		}
	}
	return nil
}

func fixtureHasUntrackedRefusal(t *testing.T, source string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "future.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	rejected := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && untrackedRefusalConstructor("future.go", file, call) {
			rejected = true
		}
		return true
	})
	return rejected
}

func refusalCodeLiteral(node ast.Node, fset *token.FileSet) (string, token.Position, bool) {
	var literal *ast.BasicLit
	switch value := node.(type) {
	case *ast.CallExpr:
		if isRefusalConstructor(value.Fun) {
			if len(value.Args) == 0 {
				return "", fset.Position(value.Pos()), false
			}
			literal, _ = value.Args[0].(*ast.BasicLit)
		} else if selector, ok := value.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "addBlocker" && len(value.Args) > 0 {
			literal, _ = value.Args[0].(*ast.BasicLit)
		}
	case *ast.CompositeLit:
		name := ""
		switch kind := value.Type.(type) {
		case *ast.Ident:
			name = kind.Name
		case *ast.SelectorExpr:
			name = kind.Sel.Name
		}
		if name != "TransitionBlocker" {
			return "", token.Position{}, false
		}
		for _, element := range value.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := field.Key.(*ast.Ident)
			if ok && name.Name == "Code" {
				literal, _ = field.Value.(*ast.BasicLit)
				break
			}
		}
	}
	if literal == nil || literal.Kind != token.STRING {
		return "", token.Position{}, false
	}
	code, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", token.Position{}, false
	}
	return code, fset.Position(literal.Pos()), true
}

func walkProductionGo(t *testing.T, visit func(string, *token.FileSet, *ast.File)) {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository from test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	internal := filepath.Join(root, "internal")
	fset := token.NewFileSet()
	err := filepath.WalkDir(internal, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		visit(filepath.ToSlash(rel), fset, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
