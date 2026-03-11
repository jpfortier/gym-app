package command

// Command types for the Command DSL.
const (
	EnsureSession      = "ENSURE_SESSION"
	CreateExerciseEntry = "CREATE_EXERCISE_ENTRY"
	AppendSet          = "APPEND_SET"
	UpdateSet          = "UPDATE_SET"
	DeleteSet          = "DELETE_SET"
	DisableEntry       = "DISABLE_ENTRY"
	RestoreEntry       = "RESTORE_ENTRY"
	SetName            = "SET_NAME"
	UpdateName         = "UPDATE_NAME"
	CreateNote         = "CREATE_NOTE"
)

// SetSpec describes a single set for CREATE_EXERCISE_ENTRY.
type SetSpec struct {
	Weight   *float64 `json:"weight"`
	Reps     int      `json:"reps"`
	SetOrder int      `json:"set_order,omitempty"`
	SetType  string   `json:"set_type,omitempty"`
}

// Command is a single DSL command. Type determines which fields are used.
type Command struct {
	Type string `json:"type"`

	// ENSURE_SESSION
	Date string `json:"date,omitempty"`

	// CREATE_EXERCISE_ENTRY (session_id optional if date given; use active session or date)
	SessionID string   `json:"session_id,omitempty"`
	Exercise  string   `json:"exercise,omitempty"`
	Variant   string   `json:"variant,omitempty"`
	RawSpeech string   `json:"raw_speech,omitempty"`
	Notes     string   `json:"notes,omitempty"`
	Sets      []SetSpec `json:"sets,omitempty"`

	// APPEND_SET, UPDATE_SET, DELETE_SET
	TargetRef string   `json:"target_ref,omitempty"` // last_created_set, last_exercise, entry_id, set_id
	Weight    *float64 `json:"weight,omitempty"`
	Reps      *int     `json:"reps,omitempty"`

	// UPDATE_SET changes
	Changes *SetChanges `json:"changes,omitempty"`

	// SET_NAME, UPDATE_NAME
	Name string `json:"name,omitempty"`

	// CREATE_NOTE
	Category string `json:"category,omitempty"`
	Content  string `json:"content,omitempty"`
}

// SetChanges for UPDATE_SET.
type SetChanges struct {
	Weight *float64 `json:"weight,omitempty"`
	Reps   *int     `json:"reps,omitempty"`
}

// ExecutionResult holds the outcome of executing commands.
type ExecutionResult struct {
	Success       bool
	CreatedEntryIDs []string
	CreatedSetIDs   []string
	UpdatedSetID    string
	RestoredEntryID string
	StoredName     string
	PRs            []PRInfo
	Error          string
}

// PRInfo describes a new PR for the LLM to celebrate.
type PRInfo struct {
	ID       string  `json:"id"`
	Exercise string  `json:"exercise"`
	Variant  string  `json:"variant"`
	Weight   float64 `json:"weight"`
	Reps     *int    `json:"reps,omitempty"`
	PRType   string  `json:"pr_type"`
}
