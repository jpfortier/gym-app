package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/env"
)

const imagesEditURL = "https://api.openai.com/v1/images/edits"

type imagesEditRequest struct {
	Model          string              `json:"model"`
	Images         []imagesEditImage   `json:"images"`
	Prompt         string              `json:"prompt"`
	Size           string              `json:"size"`
	InputFidelity  string              `json:"input_fidelity,omitempty"`
	Quality        string              `json:"quality,omitempty"`
	OutputFormat   string              `json:"output_format,omitempty"`
	N              int                 `json:"n,omitempty"`
}

type imagesEditImage struct {
	FileID string `json:"file_id,omitempty"`
}

type imagesEditResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

// buildPRImagePrompt returns a rich prompt from pr-image-prompt.md style.
func buildPRImagePrompt(exerciseName string, weight float64, reps *int, prType string, visualCues string) string {
	action := "performing " + exerciseName
	var numberCue string
	if weight > 0 {
		numberCue = fmt.Sprintf("The weights are red train freight cars with \"%.0f\" in large yellow numbers.", weight)
	} else if reps != nil {
		numberCue = fmt.Sprintf("A sign or display showing \"%d\" reps in large numbers.", *reps)
	} else {
		numberCue = "Show the PR number prominently on weights, signage, or a scoreboard."
	}
	prompt := fmt.Sprintf(`Place this character in a gritty industrial warehouse gym / train yard setting. %s. %s Concrete floor, metal, strip lights. Bold cartoon/comic style, slight vintage grunge texture. Yellow/orange train, red/blue freight cars, industrial grays. Celebrating a new personal record. Keep the character consistent with the reference images.`, action, numberCue)
	if visualCues != "" {
		cues := strings.ReplaceAll(strings.TrimSpace(visualCues), "\n", "; ")
		prompt += fmt.Sprintf(" Visual cues for this exercise (use these to depict it correctly): %s.", cues)
	}
	return prompt
}

// GeneratePRImage creates a PR celebration image using gpt-image-1.5 Edit API with reference images.
func (c *Client) GeneratePRImage(ctx context.Context, userID uuid.UUID, exerciseName string, weight float64, reps *int, prType string, visualCues string) ([]byte, error) {
	if c.testMode {
		return nil, nil
	}
	if c.client == nil {
		return nil, fmt.Errorf("openai client not configured")
	}
	if err := c.throttle.AllowDalle(ctx, userID); err != nil {
		return nil, err
	}
	refIDs := env.PRImageRefFileIDs()
	if len(refIDs) == 0 {
		return nil, fmt.Errorf("no PR image reference file IDs configured (GYM_PR_IMAGE_REF_1, GYM_PR_IMAGE_REF_2)")
	}
	images := make([]imagesEditImage, len(refIDs))
	for i, id := range refIDs {
		images[i] = imagesEditImage{FileID: id}
	}
	prompt := buildPRImagePrompt(exerciseName, weight, reps, prType, visualCues)
	body := imagesEditRequest{
		Model:         "gpt-image-1.5",
		Images:        images,
		Prompt:        prompt,
		Size:          "1024x1024",
		InputFidelity: "high",
		Quality:       "high",
		OutputFormat:  "png",
		N:             1,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal edit request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, imagesEditURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.OpenAIAPIKey())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("images edit: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("images edit %d: %s", resp.StatusCode, string(respBody))
	}
	var editResp imagesEditResponse
	if err := json.Unmarshal(respBody, &editResp); err != nil {
		return nil, fmt.Errorf("parse edit response: %w", err)
	}
	if len(editResp.Data) == 0 || editResp.Data[0].B64JSON == "" {
		return nil, fmt.Errorf("no image in edit response")
	}
	if c.usage != nil {
		cost := CostCents("gpt-image-1.5", 0, 0)
		c.usage.Record(ctx, &userID, "gpt-image-1.5", 0, 0, cost)
	}
	return base64.StdEncoding.DecodeString(editResp.Data[0].B64JSON)
}
