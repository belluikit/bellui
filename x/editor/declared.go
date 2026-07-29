package editor

// DeclaredNames is a workspace-supplied set of declared identifier names that a
// language [Highlighter] consults to color schema-informed use sites — the
// RFC-001 annotation, type, and function names discovered by an analyze pass.
//
// It is the injection point for the "middle path" between static keyword sets
// and a full semantic-token overlay (see [Editor.SetSemanticTokens]): the
// highlighter stays line-based but colors a use site like @myAnno as an
// annotation only when the name is actually declared, and a bare identifier as
// a type/function only when it names one. Consult the individual highlighter's
// SetDeclaredNames for which categories it uses.
//
// The set is line-based and advisory: it cannot disambiguate purely by
// position, and it is stale between analyze passes, so the host refreshes it on
// the analyze-completion path. Names are matched exactly (rune-for-rune),
// without the leading @ for annotations.
type DeclaredNames struct {
	// Annotations are declared annotation names, matched against @name use
	// sites (without the leading @).
	Annotations []string
	// Types are declared type names, matched against bare type references.
	Types []string
	// Functions are declared function names, matched against bare function
	// references.
	Functions []string
}

// nameSet builds a lookup set from a name slice, or nil when empty.
func nameSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	m := make(map[string]bool, len(names))
	for _, n := range names {
		if n != "" {
			m[n] = true
		}
	}
	return m
}

// DeclaredNameSets is the internal, lookup-ready form of [DeclaredNames] a
// highlighter holds. Build it with [DeclaredNames.Sets]; a zero value (all-nil
// sets) matches nothing, so a highlighter with no declared names behaves
// exactly as its static self.
type DeclaredNameSets struct {
	Annotations map[string]bool
	Types       map[string]bool
	Functions   map[string]bool
}

// Sets returns the lookup-ready form of the declared names.
func (d DeclaredNames) Sets() DeclaredNameSets {
	return DeclaredNameSets{
		Annotations: nameSet(d.Annotations),
		Types:       nameSet(d.Types),
		Functions:   nameSet(d.Functions),
	}
}
