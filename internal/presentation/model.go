package presentation

// Status is a semantic status for status lines and summaries.
type Status int

const (
	StatusNone Status = iota
	StatusSuccess
	StatusWarning
	StatusError
	StatusInfo
	StatusPending
	StatusRunning
	StatusSkipped
	StatusCancelled
)

// ValueKind selects value styling for key-value rows.
type ValueKind int

const (
	ValuePlain ValueKind = iota
	ValuePackage
	ValueVersion
	ValuePath
	ValueCommand
	ValueNumber
	ValueMuted
)

// SymbolRole is a semantic symbol role for non-status presentation symbols.
// Status roles (Success, Warning, Error, etc.) use Status + RenderSemanticSymbol.
// Mutation roles (Added, Updated, Removed), structural roles (Arrow, Bullet,
// Ellipsis, Placeholder, Separator) use SymbolRole + RenderSymbolRole.
type SymbolRole int

const (
	RoleAdded SymbolRole = iota
	RoleUpdated
	RoleRemoved
	RoleActionArrow
	RoleStructuralArrow
	RoleBullet
	RoleEllipsis
	RolePlaceholder
	RoleSeparator
)

// StatusLine is a one-line command outcome.
type StatusLine struct {
	Status Status
	Text   string
	Detail string
}

// KeyValue is one labeled field.
type KeyValue struct {
	Key   string
	Value string
	Style ValueKind
}

// Notice is a non-fatal advisory.
type Notice struct {
	Status  Status
	Message string
}

// Hint is a follow-up action suggestion.
type Hint struct {
	Message string
}

// Summary is a command outcome block.
type Summary struct {
	Status           Status
	Title            string
	CompletionFooter *CompletionFooter // rendered at bottom when non-nil
	Deltas           []PackageDelta
	Metrics          []KeyValue
	Notices          []Notice
	Hints            []Hint
}

// CompletionFooter is the final line of an install-family command.
// Rendered after all other sections. Message gets success styling; Duration gets muted.
type CompletionFooter struct {
	Message  string // "54 packages installed" or "Already up to date"
	Duration string // pre-formatted duration "[4.36s]" or ""
}

// DeltaKind classifies a package change row.
type DeltaKind int

const (
	DeltaAdded DeltaKind = iota
	DeltaUpdated
	DeltaRemoved
)

// PackageDelta is one package mutation row.
type PackageDelta struct {
	Kind    DeltaKind
	Name    string
	Version string
	From    string
	To      string
}

// PackageDeltaOptions controls how PackageDelta lists are rendered.
type PackageDeltaOptions struct {
	GroupByKind bool
	MaxRows     int // 0 means unlimited.
}

// SpecifierDelta is one specifier mutation row (e.g. lockfile importer specifier changes).
type SpecifierDelta struct {
	Importer string
	Name     string
	Kind     string // e.g. "dev", "peer", "" for prod
	Before   string
	After    string
}

// maxSummaryPackageDeltas bounds human delta lists to prevent unbounded output.
const maxSummaryPackageDeltas = 50

// ColumnAlign is table cell alignment.
type ColumnAlign int

const (
	AlignLeft ColumnAlign = iota
	AlignRight
)

// TruncatePolicy controls overflow for a column.
type TruncatePolicy int

const (
	TruncateEnd TruncatePolicy = iota
	TruncateMiddle
	TruncateWrap
)

// TableColumn defines one table column.
type TableColumn struct {
	Key      string
	Header   string
	MinWidth int
	Prefer   int
	Align    ColumnAlign
	Truncate TruncatePolicy
	Primary  bool // first column used as stacked title
	// CellStyle sets the ValueKind applied to every cell in this column.
	// ValuePlain (zero) means no semantic styling.
	CellStyle ValueKind
}

// StatusCell is a table cell that carries semantic status metadata.
// When a row provides a StatusCell for a column, the renderer applies
// the corresponding status styling (success/warning/error/etc.) instead
// of treating the value as a plain string.
type StatusCell struct {
	Text   string
	Status Status
}

// TableModel is a borderless table.
type TableModel struct {
	Columns []TableColumn
	Rows    []map[string]string
	// RowStatuses provides per-cell status metadata. When a row index and
	// column key have a StatusCell entry, the renderer applies status
	// styling (success/warning/error/etc.) instead of plain text.
	RowStatuses []map[string]StatusCell
}
