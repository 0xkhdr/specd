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
