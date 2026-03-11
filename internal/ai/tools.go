package ai

import (
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ChatTools returns the tools for the workout agent.
func ChatTools() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "reply_from_context",
				Description: "Use when you can answer the user's question from the workout context. Read-only, inert—no data changes. Use for questions like 'what did I do today?', 'what's my last bench?'. Do NOT use for logging, corrections, or any mutations—use execute_commands for those.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"message": {Type: jsonschema.String, Description: "Your response to the user. Use Markdown: **bold** for numbers/weights."},
					},
					Required: []string{"message"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_history",
				Description: "Fetch exercise history when the user's question cannot be answered from the workout context. Use when they ask about data outside the last 8 sessions, or for specific metrics (max weight, 1RM, total volume, etc.). Read-only.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"category":   {Type: jsonschema.String, Description: "Exercise category, e.g. 'bench press', 'deadlift'"},
						"variant":   {Type: jsonschema.String, Description: "Variant name, default 'standard'"},
						"scope":     {Type: jsonschema.String, Description: "most_recent, recent, best, aggregate, session_detail, trend"},
						"metric":    {Type: jsonschema.String, Description: "max_weight, latest_weight, max_reps, count_sets, count_sessions, total_volume, estimated_1rm"},
						"from_date": {Type: jsonschema.String, Description: "YYYY-MM-DD, optional"},
						"to_date":   {Type: jsonschema.String, Description: "YYYY-MM-DD, optional"},
						"limit":     {Type: jsonschema.Integer, Description: "Max entries, default 20, max 50"},
					},
					Required: []string{"category"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "execute_commands",
				Description: "Run workout commands: log exercises, append sets, update/delete sets, restore entries, set/update name, create notes. Include the success message you would show the user in the same turn; we use it if execution succeeds.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"commands": {
							Type: jsonschema.Array,
							Items: &jsonschema.Definition{
								Type: jsonschema.Object,
								Properties: map[string]jsonschema.Definition{
									"type":       {Type: jsonschema.String, Description: "ENSURE_SESSION, CREATE_EXERCISE_ENTRY, APPEND_SET, UPDATE_SET, DELETE_SET, RESTORE_ENTRY, SET_NAME, UPDATE_NAME, CREATE_NOTE"},
									"date":       {Type: jsonschema.String, Description: "YYYY-MM-DD for ENSURE_SESSION"},
									"exercise":   {Type: jsonschema.String, Description: "For CREATE_EXERCISE_ENTRY, CREATE_NOTE"},
									"variant":   {Type: jsonschema.String, Description: "For CREATE_EXERCISE_ENTRY, CREATE_NOTE. Omit when user doesn't specify—we use the standard variant for that category."},
									"raw_speech": {Type: jsonschema.String, Description: "For CREATE_EXERCISE_ENTRY"},
									"sets": {
										Type: jsonschema.Array,
										Items: &jsonschema.Definition{
											Type: jsonschema.Object,
											Properties: map[string]jsonschema.Definition{
												"weight":    {Type: jsonschema.Number},
												"reps":     {Type: jsonschema.Integer},
												"set_order": {Type: jsonschema.Integer},
												"set_type":  {Type: jsonschema.String},
											},
										},
									},
									"target_ref": {Type: jsonschema.String, Description: "last_created_set, last_exercise, or entry/set ID for APPEND_SET, UPDATE_SET, DELETE_SET"},
									"weight":     {Type: jsonschema.Number, Description: "For APPEND_SET"},
									"reps":       {Type: jsonschema.Integer, Description: "For APPEND_SET"},
									"changes": {
										Type: jsonschema.Object,
										Properties: map[string]jsonschema.Definition{
											"weight": {Type: jsonschema.Number},
											"reps":   {Type: jsonschema.Integer},
										},
										Description: "For UPDATE_SET",
									},
									"name":    {Type: jsonschema.String, Description: "For SET_NAME, UPDATE_NAME"},
									"category": {Type: jsonschema.String, Description: "For CREATE_NOTE"},
									"content": {Type: jsonschema.String, Description: "For CREATE_NOTE"},
								},
							},
						},
						"success_message": {
							Type:        jsonschema.String,
							Description: "The message to show the user if execution succeeds. Use Markdown: **bold** for numbers/weights. E.g. 'Logged bench press **140×8** for today.'",
						},
					},
					Required: []string{"commands"},
				},
			},
		},
	}
}
