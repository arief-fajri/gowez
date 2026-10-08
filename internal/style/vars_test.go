package style

import (
	"strings"
	"testing"

	"github.com/arief-fajri/gowez/internal/ui"
)

// Custom properties and var() (M6a). The semantics are the risky part: a
// substitution that resolves to the wrong value is a silent misrender, and the
// failure modes (inheritance, cycles, fallbacks) are the ones a happy-path test
// never reaches.

func resolveTree(t *testing.T, css string, build func(*ui.Tree) *ui.Node) map[ui.NodeID]ComputedStyle {
	t.Helper()
	sheet, err := Parse(css)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tree := ui.NewTree()
	root := build(tree)
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return styles
}

func TestCustomPropertyUsedInAnotherDeclaration(t *testing.T) {
	t.Parallel()
	styles := resolveTree(t, `
.app { --brand: #ff0000; color: var(--brand) }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "app")
		return n
	})
	if got := styles[1].Color; got != 0xff0000ff {
		t.Fatalf("color = %#08x, want 0xff0000ff", got)
	}
}

func TestCustomPropertyInheritsItsComputedValue(t *testing.T) {
	t.Parallel()
	// What inherits is the **computed** value of a custom property — that is,
	// its value after substitution (CSS Variables 1 §2.2). So `--ink` on the
	// parent computes to #ffffff, and that substituted text is what reaches the
	// child; overriding `--fg` below does not retroactively change it.
	//
	// This is worth pinning deliberately rather than accidentally: it is
	// surprising, it is the opposite of substituting at use time, and getting it
	// wrong would make `--ink: var(--fg)` theme-independent.
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	root.SetAttribute("class", "app")
	child := tree.CreateElement("div")
	child.SetAttribute("class", "dark")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := tree.Append(root, child); err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(`
.app { --fg: #ffffff; --ink: var(--fg) }
.dark { --fg: #000000; color: var(--ink) }
`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if got := styles[child.ID].Color; got != 0xffffffff {
		t.Fatalf("descendant color = %#08x, want 0xffffffff (the computed --ink is inherited)", got)
	}
	if got := styles[child.ID].Custom["--ink"]; got != "#ffffff" {
		t.Fatalf("inherited --ink = %q, want %q (substitution happens before inheritance)", got, "#ffffff")
	}
}

func TestCustomPropertyDeclaredOnChildIsNotAffectedByAncestorOverride(t *testing.T) {
	t.Parallel()
	// The counterpart: a descendant that declares its own var() chain resolves
	// against its own scope, which is how a theme override actually works.
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	root.SetAttribute("class", "app")
	child := tree.CreateElement("div")
	child.SetAttribute("class", "dark")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := tree.Append(root, child); err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(`
.app { --fg: #ffffff }
.dark { --fg: #000000; --ink: var(--fg); color: var(--ink) }
`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if got := styles[child.ID].Color; got != 0x000000ff {
		t.Fatalf("descendant color = %#08x, want 0x000000ff", got)
	}
}

func TestCustomPropertyCascadeOrderDefinesTheWinner(t *testing.T) {
	t.Parallel()
	styles := resolveTree(t, `
.app { --ink: #111111 }
.app { --ink: #222222 }
.card { color: var(--ink) }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "app card")
		return n
	})
	// Same specificity, later declaration wins — for custom properties as much
	// as for ordinary ones.
	if got := styles[1].Color; got != 0x222222ff {
		t.Fatalf("color = %#08x, want 0x222222ff (later declaration must win)", got)
	}
}

func TestVarFallbackWhenUndefined(t *testing.T) {
	t.Parallel()
	styles := resolveTree(t, `
.card { color: var(--missing, #00ff00) }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "card")
		return n
	})
	if got := styles[1].Color; got != 0x00ff00ff {
		t.Fatalf("color = %#08x, want 0x00ff00ff (fallback must be used)", got)
	}
}

func TestVarFallbackWhenDefinedButEmpty(t *testing.T) {
	t.Parallel()
	// An empty custom property counts as unset, so the fallback applies.
	// Substituting the empty string instead would yield a parse failure or,
	// worse, an empty value that "succeeds".
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseInline("--ink: ; color: var(--ink, #00ff00)"); err != nil {
		t.Fatalf("parse inline: %v", err)
	}
	root.SetAttribute("style", "--ink: ; color: var(--ink, #00ff00)")
	styles, err := Resolve(tree.Roots(), mustSheet(t, "div { color: #0000ff }"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := styles[root.ID].Color; got != 0x00ff00ff {
		t.Fatalf("color = %#08x, want 0x00ff00ff (empty --ink must use the fallback)", got)
	}
}

func TestVarUndefinedWithoutFallbackIsAnError(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(`div { color: var(--nope) }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(tree.Roots(), sheet); err == nil {
		t.Fatal("resolve accepted var(--nope) with no fallback and no definition")
	} else if !strings.Contains(err.Error(), "var(--nope) is not defined") {
		t.Fatalf("error = %v, want it to name the undefined property", err)
	}
}

func TestCustomPropertyCycleIsAnErrorNotAHang(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(`div { --a: var(--b); --b: var(--a); color: var(--a) }`)
	if err != nil {
		t.Fatal(err)
	}
	// There is no least fixed point, so there is nothing to substitute. The
	// alternative — resolving to empty — is a silent misrender.
	_, err = Resolve(tree.Roots(), sheet)
	if err == nil {
		t.Fatal("resolve accepted a custom property cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v, want it to name the cycle", err)
	}
}

func TestVarInsideNestedFallback(t *testing.T) {
	t.Parallel()
	// var(--a, var(--b, #00ff00)) — the fallback itself contains var(), and its
	// own comma must not be mistaken for the outer separator.
	styles := resolveTree(t, `
.card { color: var(--a, var(--b, #00ff00)) }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "card")
		return n
	})
	if got := styles[1].Color; got != 0x00ff00ff {
		t.Fatalf("color = %#08x, want 0x00ff00ff", got)
	}
}

func TestVarSubstitutesInsideMultipleValues(t *testing.T) {
	t.Parallel()
	// A var() reference is one token among several: splitting on whitespace
	// before substitution would break this.
	styles := resolveTree(t, `
.app { --gap: 8px; padding: var(--gap) 12px }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "app")
		return n
	})
	if got, want := styles[1].Padding, [4]float64{8, 12, 8, 12}; got != want {
		t.Fatalf("padding = %v, want %v", got, want)
	}
}

func TestCustomPropertyValueIsNotParsedUntilConsumed(t *testing.T) {
	t.Parallel()
	// `--x` holds a token stream that is only meaningful to the property that
	// consumes it. Parsing it at declaration time would make this illegal.
	styles := resolveTree(t, `
.app { --line: 2px; border-bottom-width: var(--line) }
`, func(tr *ui.Tree) *ui.Node {
		n := tr.CreateElement("div")
		n.SetAttribute("class", "app")
		return n
	})
	if got := styles[1].BorderWidth[2]; got != 2 {
		t.Fatalf("border-bottom-width = %v, want 2", got)
	}
}

func TestSubstitutedValueStillGetsTypedValidation(t *testing.T) {
	t.Parallel()
	// Deferring validation to resolve must not weaken it: a custom property
	// holding nonsense for the property that uses it is still an error.
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(`div { --gap: banana; padding: var(--gap) }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(tree.Roots(), sheet); err == nil {
		t.Fatal("resolve accepted padding: var(--gap) where --gap is not a length")
	}
}

func TestColorMixPremultipliesAlpha(t *testing.T) {
	t.Parallel()
	// The classic error: interpolating alpha independently gives #80000080
	// instead of #ff000080.
	got, err := parseColorValue("color-mix(in srgb, #ff0000 50%, transparent)")
	if err != nil {
		t.Fatal(err)
	}
	want := uint32(0xff000080)
	if got != want {
		t.Fatalf("color-mix = %#08x, want %#08x", got, want)
	}
}

func TestColorMixPercentagesAndOmittedPercentage(t *testing.T) {
	t.Parallel()
	// In `color-mix(in srgb, A p%, B)` the percentage is A's share; B takes the
	// remainder. With no percentage at all it is an even split.
	for _, tc := range []struct {
		value string
		want  uint32
	}{
		{"color-mix(in srgb, #000000, #ffffff)", 0x808080ff},
		{"color-mix(in srgb, #000000 25%, #ffffff)", 0xbfbfbfff},
		{"color-mix(in srgb, #000000 75%, #ffffff)", 0x404040ff},
		{"color-mix(in srgb, #ffffff 0%, #000000)", 0x000000ff},
		{"color-mix(in srgb, #ffffff 100%, #000000)", 0xffffffff},
	} {
		got, err := parseColorValue(tc.value)
		if err != nil {
			t.Errorf("%s: %v", tc.value, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %#08x, want %#08x", tc.value, got, tc.want)
		}
	}
}

func TestColorMixRejectsOtherColorSpacesByName(t *testing.T) {
	t.Parallel()
	// A refusal that names the space is honest; accepting it and mixing in srgb
	// anyway would return a different colour from every browser.
	_, err := parseColorValue("color-mix(in oklch, #000, #fff)")
	if err == nil {
		t.Fatal("color-mix accepted a colour space the subset does not implement")
	}
	if !strings.Contains(err.Error(), "in srgb") {
		t.Fatalf("error = %v, want it to name the supported space", err)
	}
}

func TestColorMixRejectsPercentagesThatDoNotSumTo100(t *testing.T) {
	t.Parallel()
	if _, err := parseColorValue("color-mix(in srgb, #000 30%, #fff 30%)"); err == nil {
		t.Fatal("color-mix accepted percentages that do not sum to 100")
	}
}

func TestColorMixInsideBorderColor(t *testing.T) {
	t.Parallel()
	// The dashboard's real declaration.
	sheet, err := Parse(`.x { border-color: color-mix(in srgb, #dc2626 45%, transparent) }`)
	if err != nil {
		t.Fatal(err)
	}
	ids := sheet.Rules[0].Declarations
	if len(ids) != 4 {
		t.Fatalf("border-color expanded to %d longhands, want 4", len(ids))
	}
	for _, d := range ids {
		if d.Property != "border-top-color" && d.Property != "border-right-color" &&
			d.Property != "border-bottom-color" && d.Property != "border-left-color" {
			t.Errorf("unexpected longhand %q", d.Property)
		}
	}
}

func TestNoneOnlyPropertiesRejectEverythingElse(t *testing.T) {
	t.Parallel()
	// `none` is true because the subset paints neither; anything else must be
	// refused rather than swallowed. Driven through Parse, which is the path an
	// author's stylesheet actually takes.
	for _, ok := range []struct{ prop, value string }{
		{"list-style", "none"},
		{"outline", "none"},
	} {
		if _, err := Parse(`.x { ` + ok.prop + `: ` + ok.value + ` }`); err != nil {
			t.Errorf("%s: %s: %v", ok.prop, ok.value, err)
		}
	}
	for _, bad := range []struct{ prop, value string }{
		{"list-style", "disc"},
		{"list-style", "inside"},
		{"outline", "2px solid red"},
		{"outline", "auto"},
	} {
		if _, err := Parse(`.x { ` + bad.prop + `: ` + bad.value + ` }`); err == nil {
			t.Errorf("%s: %s was accepted; the subset paints neither and must say so",
				bad.prop, bad.value)
		}
	}
}

func TestCustomPropertyResolutionDoesNotDependOnMapOrder(t *testing.T) {
	t.Parallel()
	// A chain resolved in one pass made the answer depend on Go's map iteration
	// order: `--ink: var(--fg)` failed with "not defined" whenever `--ink` was
	// visited before `--fg`. It passed on most runs and failed intermittently
	// under -count, which is the worst possible shape for a test.
	//
	// A three-link chain forces the dependent property to be visited first in
	// some orderings and last in others; -count=25 makes an order-dependent
	// implementation fail reliably.
	const css = `
.app {
  --fg: #ffffff;
  --mid: var(--fg);
  --ink: var(--mid);
  color: var(--ink);
}`
	for i := 0; i < 25; i++ {
		styles := resolveTree(t, css, func(tr *ui.Tree) *ui.Node {
			n := tr.CreateElement("div")
			n.SetAttribute("class", "app")
			return n
		})
		if got := styles[1].Color; got != 0xffffffff {
			t.Fatalf("iteration %d: color = %#08x, want 0xffffffff", i, got)
		}
	}
}

func TestCustomPropertyChainAndCycleTogether(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	// --b is fine, but --a and --c form a cycle reachable through --b.
	sheet, err := Parse(`div { --a: var(--c); --b: var(--a); --c: var(--a); color: var(--b) }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(tree.Roots(), sheet); err == nil {
		t.Fatal("resolve accepted an indirect custom property cycle")
	} else if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v, want it to name the cycle", err)
	}
}

func mustSheet(t *testing.T, css string) *Stylesheet {
	t.Helper()
	sh, err := Parse(css)
	if err != nil {
		t.Fatal(err)
	}
	return sh
}
