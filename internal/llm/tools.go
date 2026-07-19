package llm

import "github.com/anthropics/anthropic-sdk-go"

const (
	ToolWriteFile    = "write_file"
	ToolSetPlan      = "set_plan"
	ToolSubmitReview = "submit_review"
)

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
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

func toolDefinitions() []anthropic.ToolUnionParam {
	writeFile := anthropic.ToolParam{
		Name:        ToolWriteFile,
		Description: anthropic.String("Write a file in the learning project workspace. Only permitted when the typing dial allows it; otherwise the call is rejected."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path":    map[string]any{"type": "string", "description": "Relative path within the workspace"},
				"content": map[string]any{"type": "string", "description": "Full file content"},
			},
			Required: []string{"path", "content"},
		},
	}
	setPlan := anthropic.ToolParam{
		Name:        ToolSetPlan,
		Description: anthropic.String("Set the task plan for the session: a short project title and 3-5 small, ordered steps the learner will implement."),
		InputSchema: anthropic.ToolInputSchemaParam{
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
	}
	submitReview := anthropic.ToolParam{
		Name:        ToolSubmitReview,
		Description: anthropic.String("Submit the verdict after reviewing the learner's diff against the current step's goal. Use 'pass' when the goal is met by any valid approach, 'retry' when it is not."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"verdict":  map[string]any{"type": "string", "enum": []string{"pass", "retry"}},
				"feedback": map[string]any{"type": "string", "description": "Teaching feedback for the learner"},
			},
			Required: []string{"verdict", "feedback"},
		},
	}
	return []anthropic.ToolUnionParam{
		{OfTool: &writeFile},
		{OfTool: &setPlan},
		{OfTool: &submitReview},
	}
}
