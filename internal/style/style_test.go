package style

import (
	"strings"
	"testing"

	"github.com/arief-fajri/gowez/internal/ui"
)

func TestParseValidSubset(t *testing.T) {
	t.Parallel()
	const src = `
/* card styles */
.card, #title {
	margin: 12px 8px;
	padding: 4px;
	border-width: 1px;
	border-color: #ccc;
	background-color: #1e2028;
	color: white;
	font-size: 15px;
	display: flex;
	flex-direction: column;
	justify-content: space-between;
	align-items: center;
	gap: 8px;
	flex-grow: 1;
	flex-shrink: 0;
	width: 50%;
	height: 100px;
}
`
	sheet, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Rules) != 2 {
		t.Fatalf("rules = %d, want 2 (group expanded)", len(sheet.Rules))
	}
	if classes := sheet.Rules[0].Selector.Steps[0].Classes; len(classes) != 1 || classes[0] != "card" {
		t.Fatalf("first selector classes = %v, want [card]", classes)
	}
	if got := sheet.Rules[1].Selector.Steps[0].ID; got != "title" {
		t.Fatalf("second selector id = %q, want %q", got, "title")
	}
	if id, cls, typ := sheet.Rules[1].Selector.specificity(); id != 1 || cls != 0 || typ != 0 {
		t.Fatalf("specificity = (%d,%d,%d), want (1,0,0)", id, cls, typ)
	}

	// Shorthand expands to longhands before the cascade ever sees them.
	decls := sheet.Rules[0].Declarations
	want := map[string]string{
		"margin-top": "12px", "margin-right": "8px",
		"margin-bottom": "12px", "margin-left": "8px",
		"padding-top": "4px", "width": "50%", "height": "100px",
	}
	for _, d := range decls {
		delete(want, d.Property)
		if d.Property == "margin-top" && d.Value != "12px" {
			t.Fatalf("margin-top = %q, want 12px", d.Value)
		}
	}
	for prop := range want {
		t.Errorf("missing declaration %q", prop)
	}
}

func TestParseShorthandExpansion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value string
		want  [4]string
	}{
		{"4px", [4]string{"4px", "4px", "4px", "4px"}},
		{"4px 8px", [4]string{"4px", "8px", "4px", "8px"}},
		{"1px 2px 3px", [4]string{"1px", "2px", "3px", "2px"}},
		{"1px 2px 3px 4px", [4]string{"1px", "2px", "3px", "4px"}},
		{"0", [4]string{"0px", "0px", "0px", "0px"}},
	}
	for _, c := range cases {
		sheet, err := Parse("div { margin: " + c.value + " }")
		if err != nil {
			t.Fatalf("Parse(margin: %s): %v", c.value, err)
		}
		got := [4]string{}
		for _, d := range sheet.Rules[0].Declarations {
			switch d.Property {
			case "margin-top":
				got[0] = d.Value
			case "margin-right":
				got[1] = d.Value
			case "margin-bottom":
				got[2] = d.Value
			case "margin-left":
				got[3] = d.Value
			}
		}
		if got != c.want {
			t.Errorf("margin %s → %v, want %v", c.value, got, c.want)
		}
	}
}

func TestParseRejectsUnsupported(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, src, want string }{
		{"universal selector", `* { color: #fff }`, "unsupported selector syntax"},
		{"child combinator", `div > span { color: #fff }`, "unsupported selector syntax"},
		{"sibling combinator", `h1 + p { color: #fff }`, "unsupported selector syntax"},
		{"unsupported pseudo-class", `a:focus-visible { color: #fff }`, "unsupported pseudo-class"},
		{"pseudo-element", `a::before { color: #fff }`, "unsupported pseudo-element"},
		{"attribute selector", `a[href] { color: #fff }`, "unsupported selector syntax"},
		{"at-rule", `@media screen { div { color: #fff } }`, "unsupported selector syntax"},
		{"important", `div { color: #fff !important }`, "!important is not supported"},
		{"unknown property", `div { border-radius: 4px }`, "unsupported property"},
		{"unknown value", `div { display: grid }`, "unsupported value"},
		{"percentage height", `div { height: 50% }`, "percentage heights are not supported"},
		{"negative length", `div { margin: -4px }`, "negative lengths are not supported"},
		{"missing unit", `div { width: 10 }`, "want a px length"},
		{"bare color keyword", `div { color: red }`, "unsupported color"},
		{"bad hex", `div { color: #12345 }`, "want #RGB"},
		{"zero font-size", `div { font-size: 0 }`, "must be positive"},
		{"missing colon", `div { color: #fff; padding }`, "expected ':'"},
		{"missing value", `div { color: }`, "missing value"},
		{"missing brace", `div { color: #fff`, "unterminated block"},
		{"unterminated comment", `div { color: #fff } /* oops`, "unterminated comment"},
		{"empty group member", `div, { color: #fff }`, "empty selector"},
		{"negative flex", `div { flex-grow: -1 }`, "negative numbers are not supported"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(c.src)
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want %q", c.src, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Parse(%q) error = %q, want it to contain %q", c.src, err, c.want)
			}
			if !strings.Contains(err.Error(), "style: line") {
				t.Fatalf("Parse(%q) error = %q, want a line:col position", c.src, err)
			}
		})
	}
}

func TestParseErrorPosition(t *testing.T) {
	t.Parallel()
	_, err := Parse("div {\n\tcolor: #fff;\n\tbogus: 1;\n}")
	if err == nil {
		t.Fatal("want error for unknown property")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error = %q, want line 3", err)
	}
}

func TestParseInline(t *testing.T) {
	t.Parallel()
	decls, err := ParseInline("color: #fff; font-size: 12px")
	if err != nil {
		t.Fatalf("ParseInline: %v", err)
	}
	if len(decls) != 2 || decls[0].Property != "color" || decls[1].Property != "font-size" {
		t.Fatalf("decls = %+v", decls)
	}
	if _, err := ParseInline("color: red"); err == nil {
		t.Fatal("ParseInline(bad color) = nil error")
	}
	if _, err := ParseInline("bogus: 1"); err == nil {
		t.Fatal("ParseInline(unknown property) = nil error")
	}
}

// buildStyleTree builds:
//
//	root (div.card)
//	└── title (h1#t.big)
//	    └── "Hello" (text)
//	and a sibling div.other.
func buildStyleTree(t *testing.T) (*ui.Tree, *ui.Node) {
	t.Helper()
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	root.SetAttribute("class", "card")
	title := tree.CreateElement("h1")
	title.SetAttribute("id", "t")
	title.SetAttribute("class", "big")
	text := tree.CreateText("Hello")
	other := tree.CreateElement("div")
	other.SetAttribute("class", "other")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*ui.Node{
		{root, title}, {title, text}, {root, other},
	} {
		if err := tree.Append(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	return tree, title
}

func TestSelectorMatches(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	textNode := title.Children[0]
	root := tree.Roots()[0]

	cases := []struct {
		selector string
		want     bool
		node     *ui.Node
	}{
		{"h1", true, title},
		{"div", true, root},
		{"h1.big", true, title},
		{"#t", true, title},
		{".big", true, title},
		{".card h1", true, title},
		{".card .big", true, title},
		{"div h1", true, title},
		{"h1 text", true, textNode},
		{"text", true, textNode},
		{".missing", false, title},
		{"#other", false, title},
		{"span", false, title},
		{".card span", false, title},
		{"h1 h1", false, title},
		{".other h1", false, title},
		{"card", false, title}, // type "card" ≠ tag "div"
	}
	for _, c := range cases {
		sheet, err := Parse(c.selector + " { color: #fff }")
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.selector, err)
		}
		if got := sheet.Rules[0].Selector.Matches(c.node); got != c.want {
			t.Errorf("%q matches node %d = %v, want %v", c.selector, c.node.ID, got, c.want)
		}
	}
}

func TestResolveInitialValues(t *testing.T) {
	t.Parallel()
	tree, _ := buildStyleTree(t)
	styles, err := Resolve(tree.Roots())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	st, ok := styles[tree.Roots()[0].ID]
	if !ok {
		t.Fatal("root missing from styles")
	}
	if st.Display != DisplayBlock || st.Color != 0x000000ff || st.FontSize != 16 {
		t.Fatalf("initial style = %+v", st)
	}
	if st.Width.Kind != LengthAuto || st.Height.Kind != LengthAuto {
		t.Fatalf("initial size = %v/%v, want auto", st.Width, st.Height)
	}
	if st.FlexShrink != 1 || st.AlignItems != AlignStretch {
		t.Fatalf("initial flex = %+v", st)
	}
	if len(styles) != 4 {
		t.Fatalf("styles entries = %d, want 4", len(styles))
	}
}

func TestResolveCascade(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse(`
div { color: #111111 }
.card { color: #222222 }
h1 { color: #aaaaaa }
#t { color: #333333 }
.big { color: #444444 }
`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	// #id (1,0,0) beats .class (0,1,0) beats type (0,0,1).
	if got := styles[title.ID].Color; got != 0x333333ff {
		t.Fatalf("title color = #%08x, want #333333ff", got)
	}
	// Root div.card: .class beats type.
	if got := styles[tree.Roots()[0].ID].Color; got != 0x222222ff {
		t.Fatalf("root color = #%08x, want #222222ff", got)
	}
}

func TestResolveSourceOrderTieBreak(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse(".big { color: #111111 } .big { color: #222222 }")
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if got := styles[title.ID].Color; got != 0x222222ff {
		t.Fatalf("color = #%08x, want later rule #222222ff", got)
	}
}

func TestResolveInlineBeatsID(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse("#t { color: #111111 }")
	if err != nil {
		t.Fatal(err)
	}
	title.SetAttribute("style", "color: #555555")
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if got := styles[title.ID].Color; got != 0x555555ff {
		t.Fatalf("color = #%08x, want inline #555555ff", got)
	}
}

func TestResolveShorthandAndDisplayNone(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse(`div { margin: 12px 8px; padding: 1px 2px 3px 4px }
h1 { display: none }`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	root := styles[tree.Roots()[0].ID]
	if root.Margin != [4]float64{12, 8, 12, 8} {
		t.Fatalf("margin = %v", root.Margin)
	}
	if root.Padding != [4]float64{1, 2, 3, 4} {
		t.Fatalf("padding = %v", root.Padding)
	}
	if got := styles[title.ID].Display; got != DisplayNone {
		t.Fatalf("title display = %v, want DisplayNone", got)
	}
}

func TestResolveInlineErrorNamesNode(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	title.SetAttribute("style", "color: chartreuse")
	_, err := Resolve(tree.Roots())
	if err == nil {
		t.Fatal("Resolve = nil error, want inline style failure")
	}
	if !strings.Contains(err.Error(), "inline style") || !strings.Contains(err.Error(), "node") {
		t.Fatalf("error = %q, want it to name the inline style and the node", err)
	}
}

func TestResolveInheritedProperties(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse(`
.card { color: #112233; font-size: 20px; }
h1 { color: #445566; }
`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	// color and font-size inherit; an explicit rule on the node wins.
	if got := styles[title.ID].Color; got != 0x445566ff {
		t.Errorf("title color = #%08x, want explicit #445566ff", got)
	}
	if got := styles[title.ID].FontSize; got != 20 {
		t.Errorf("title font size = %g, want inherited 20", got)
	}
	textNode := title.Children[0]
	if got := styles[textNode.ID].Color; got != 0x445566ff {
		t.Errorf("text color = #%08x, want inherited #445566ff", got)
	}
	if got := styles[textNode.ID].FontSize; got != 20 {
		t.Errorf("text font size = %g, want inherited 20", got)
	}
}

func TestResolveEmpty(t *testing.T) {
	t.Parallel()
	styles, err := Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve(nil): %v", err)
	}
	if len(styles) != 0 {
		t.Fatalf("styles = %d entries, want 0", len(styles))
	}
}

// --- Milestone 3: pseudo-classes ---

func TestParsePseudoClasses(t *testing.T) {
	t.Parallel()
	sheet, err := Parse(`
button:hover { color: #111111 }
:focus { color: #222222 }
button:hover:active { color: #333333 }
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Rules) != 3 {
		t.Fatalf("rules = %d, want 3", len(sheet.Rules))
	}
	if p := sheet.Rules[0].Selector.Steps[0].Pseudo; len(p) != 1 || p[0] != "hover" {
		t.Fatalf("button:hover pseudo = %v, want [hover]", p)
	}
	if p := sheet.Rules[1].Selector.Steps[0].Pseudo; len(p) != 1 || p[0] != "focus" {
		t.Fatalf(":focus pseudo = %v, want [focus]", p)
	}
	if p := sheet.Rules[2].Selector.Steps[0].Pseudo; len(p) != 2 || p[0] != "hover" || p[1] != "active" {
		t.Fatalf("button:hover:active pseudo = %v, want [hover active]", p)
	}
	// Each pseudo-class counts like a class (CSS specificity).
	if id, cls, typ := sheet.Rules[0].Selector.specificity(); id != 0 || cls != 1 || typ != 1 {
		t.Fatalf("button:hover specificity = (%d,%d,%d), want (0,1,1)", id, cls, typ)
	}
	if id, cls, typ := sheet.Rules[2].Selector.specificity(); id != 0 || cls != 2 || typ != 1 {
		t.Fatalf("button:hover:active specificity = (%d,%d,%d), want (0,2,1)", id, cls, typ)
	}
	if id, cls, typ := sheet.Rules[1].Selector.specificity(); id != 0 || cls != 1 || typ != 0 {
		t.Fatalf(":focus specificity = (%d,%d,%d), want (0,1,0)", id, cls, typ)
	}
}

// TestSelectorMatchesPseudoState proves pseudo-class matching reads the
// node's interaction state; all pseudo-classes in a compound must hold.
func TestSelectorMatchesPseudoState(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	_ = tree
	cases := []struct {
		selector string
		state    ui.StateBits
		want     bool
	}{
		{"h1:hover", ui.StateHovered, true},
		{"h1:hover", 0, false},
		{"h1:active", ui.StatePressed, true},
		{"h1:focus", ui.StateFocused, true},
		{"h1:focus", ui.StateHovered, false},
		{"h1:hover:active", ui.StateHovered | ui.StatePressed, true},
		{"h1:hover:active", ui.StateHovered, false},
		{"div:hover", ui.StateHovered, false}, // type mismatch
	}
	for _, c := range cases {
		sheet, err := Parse(c.selector + " { color: #fff }")
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.selector, err)
		}
		title.State = c.state
		if got := sheet.Rules[0].Selector.Matches(title); got != c.want {
			t.Errorf("%q (state=%d) matches = %v, want %v", c.selector, c.state, got, c.want)
		}
	}
	title.State = 0
}

// TestResolvePseudoCascade: the base rule applies when the state is
// off; a matching pseudo-class rule wins by specificity, and among
// equal-specificity pseudo rules source order decides (I6: the
// cascade never changes silently).
func TestResolvePseudoCascade(t *testing.T) {
	t.Parallel()
	tree, title := buildStyleTree(t)
	sheet, err := Parse(`
h1 { color: #111111 }
h1:hover { color: #222222 }
h1:focus { color: #333333 }
`)
	if err != nil {
		t.Fatal(err)
	}
	resolve := func() uint32 {
		styles, err := Resolve(tree.Roots(), sheet)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		return styles[title.ID].Color
	}

	if got := resolve(); got != 0x111111ff {
		t.Fatalf("no state: color = #%08x, want base #111111ff", got)
	}
	title.State = ui.StateHovered
	if got := resolve(); got != 0x222222ff {
		t.Fatalf("hover: color = #%08x, want #222222ff", got)
	}
	title.State = ui.StateFocused
	if got := resolve(); got != 0x333333ff {
		t.Fatalf("focus: color = #%08x, want #333333ff", got)
	}
	// Equal specificity → later source order (:focus) wins.
	title.State = ui.StateHovered | ui.StateFocused
	if got := resolve(); got != 0x333333ff {
		t.Fatalf("hover+focus: color = #%08x, want #333333ff (source order)", got)
	}
	// State off again → base rule.
	title.State = 0
	if got := resolve(); got != 0x111111ff {
		t.Fatalf("state cleared: color = #%08x, want base #111111ff", got)
	}
}
