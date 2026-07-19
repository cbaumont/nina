package llm

const (
	ToolWriteFile    = "write_file"
	ToolSetPlan      = "set_plan"
	ToolSubmitReview = "submit_review"
	ToolRunCommand   = "run_command"
	ToolReadFile     = "read_file"
)

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type RunCommandInput struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type ReadFileInput struct {
	Path string `json:"path"`
}

type PlanStep struct {
	Title string `json:"title"`
	Goal  string `json:"goal"`
}

type SetPlanInput struct {
	Title string     `json:"title"`
	Steps []PlanStep `json:"steps"`
}

type SubmitReviewInput struct {
	Verdict  string `json:"verdict"`
	Feedback string `json:"feedback"`
}

type ToolDef struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

func ToolDefs() []ToolDef {
	return []ToolDef{
		{
			Name:        ToolWriteFile,
			Description: "Write a file in the learning project workspace. Only permitted when the typing dial allows it; otherwise the call is rejected.",
			Properties: map[string]any{
				"path":    map[string]any{"type": "string", "description": "Relative path within the workspace"},
				"content": map[string]any{"type": "string", "description": "Full file content"},
			},
			Required: []string{"path", "content"},
		},
		{
			Name:        ToolSetPlan,
			Description: "Set the task plan for the session: a short project title and 3-5 small, ordered steps the learner will implement.",
			Properties: map[string]any{
				"title": map[string]any{"type": "string", "description": "Short project title"},
				"steps": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title": map[string]any{"type": "string", "description": "Short step title"},
							"goal":  map[string]any{"type": "string", "description": "What the learner's code must achieve for this step to pass review"},
						},
						"required": []string{"title", "goal"},
					},
				},
			},
			Required: []string{"title", "steps"},
		},
		{
			Name:        ToolRunCommand,
			Description: "Propose a shell command to run in the workspace (run code, tests, installs, version checks). The learner confirms before it executes. Returns the exit code and combined stdout/stderr. Use it to verify the learner's code by running it or its tests, and for environment setup while scaffolding.",
			Properties: map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to run in the workspace root"},
				"reason":  map[string]any{"type": "string", "description": "One short sentence shown to the learner explaining why"},
			},
			Required: []string{"command", "reason"},
		},
		{
			Name:        ToolReadFile,
			Description: "Read a file from the workspace to see its current content, for example when a diff alone lacks context during review.",
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative path within the workspace"},
			},
			Required: []string{"path"},
		},
		{
			Name:        ToolSubmitReview,
			Description: "Submit the verdict after reviewing the learner's diff against the current step's goal. Use 'pass' when the goal is met by any valid approach, 'retry' when it is not.",
			Properties: map[string]any{
				"verdict":  map[string]any{"type": "string", "enum": []string{"pass", "retry"}},
				"feedback": map[string]any{"type": "string", "description": "Teaching feedback for the learner"},
			},
			Required: []string{"verdict", "feedback"},
		},
	}
}
