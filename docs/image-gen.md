# Build PR image generation with reusable reference images (Go + OpenAI Responses API)

## Goal

Generate gym PR reward images from:

- a small runtime payload like:
  - `exercise = "rack pull"`
  - `weight_lbs = 370`
- a fixed set of pre-uploaded reference images
- a fixed reusable prompt template

Do not re-upload the same reference images every request.

Use:

- **Files API** to upload the reference images once and keep their `file_ids`
- **Responses API** with the `image_generation` tool to generate the final image
- a stored prompt template ID for reusable instructions and variables

The Responses API is the correct choice here because it supports image generation in multi-step flows and accepts image inputs by File ID.

## Why this architecture

Use this exact pattern:

1. Upload the mascot/reference images once with the Files API.
2. Save the returned `file_ids` in our database.
3. Create one reusable prompt template in OpenAI.
4. At runtime, send only:
   - the prompt template ID
   - the saved reference `file_ids`
   - the dynamic fields: exercise name and weight
5. Generate one fresh image per PR event.

This avoids re-uploading images, keeps requests small, and lets the model create a new image each time instead of swapping numbers into an old image. OpenAI documents that uploaded files can be reused across endpoints, and that Responses can take image inputs as `file_ids`.

## OpenAI features to use

### 1) Files API

Use the Files API to upload reference images once. OpenAI says uploaded files can be used across various endpoints, and the returned file object includes an `id` that can be referenced later.

### 2) Responses API + image_generation tool

Use the Responses API with `tools: [{ "type": "image_generation" }]`. OpenAI documents this as the way to generate images inside a response flow.

### 3) Prompt template

Use a stored prompt template and pass variables at runtime. The Responses API supports `prompt.id`, optional `version`, and `variables`, and those variables can include strings, image inputs, and file inputs.

## Do not use as the main design

Do not depend on a giant persistent chat thread as the canonical store for this feature.

Use:

- your own database for config
- OpenAI `file_ids` for reference images
- OpenAI prompt template ID for reusable instructions

Only use response chaining later if you specifically want iterative edits. OpenAI does support `previous_response_id`, but that is not needed for the core PR generation flow.

## What to store in our database

Create a table for the image profile.

```sql
create table pr_image_profile (
  id text primary key,
  name text not null,
  prompt_template_id text not null,
  prompt_template_version text null,
  reference_file_ids jsonb not null,
  active boolean not null default true
);
```

Example row:

```json
{
  "id": "yellow-train-pr",
  "name": "Yellow Train PR",
  "prompt_template_id": "pmpt_123",
  "prompt_template_version": "3",
  "reference_file_ids": [
    "file_ref_1",
    "file_ref_2",
    "file_ref_3",
    "file_ref_4"
  ],
  "active": true
}
```

No other persistence is required for the first version.

## What the runtime API should accept

Create one backend endpoint:

**POST /internal/pr-image**

Request body:

```json
{
  "profile_id": "yellow-train-pr",
  "exercise": "rack pull",
  "weight_lbs": 370
}
```

That is the only dynamic input needed right now.

## Prompt template to create in OpenAI

Create one prompt template in OpenAI and store its ID in the database.

Use this as the prompt content:

```
Generate a celebratory mobile-friendly PR reward image for a gym app.

Style and character:
- Use the attached reference images as the visual anchor for character design, color palette, facial expression style, locomotive body style, muscular arm style, and overall vibe.
- Keep the yellow train mascot consistent with the references.
- Make the image bold, rewarding, energetic, and instantly readable on a phone screen.
- It should feel like a high-energy reward screen someone would want to screenshot and share.

Exercise to depict:
- Exercise: {{exercise}}
- Weight: {{weight_lbs}} lb

Requirements:
- The scene must clearly depict the named exercise correctly, not a generic gym movement.
- If the exercise is rack pull, show a rack pull setup and not a full deadlift from the floor.
- Show the weight number prominently somewhere natural in the image.
- The exact placement of the number can vary: on plates, train number, gym signage, scoreboard, cargo car, or another visually strong location.
- The result should be a fresh composition each time, not a duplicate layout.
- Keep the final image clean, punchy, and easy to read.
- Vertical poster style for a phone screen.
```

This keeps the fixed style instructions in OpenAI and leaves only `exercise` and `weight_lbs` as runtime variables.

## Upload the reference images once

Use the Files API one time for each reference image and save the returned `file_id`.

OpenAI says the file upload endpoint returns a file ID and that uploaded files can be used across endpoints.

Example with curl:

```bash
curl https://api.openai.com/v1/files \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F purpose="user_data" \
  -F file="@./references/character-sheet.png"
```

Do that for each reference image.

Save the `id` from each response.

Use `purpose="user_data"` because OpenAI lists it as the flexible file type for general use.

## Runtime generation request

At runtime:

1. Load the profile from the DB.
2. Read:
   - `prompt_template_id`
   - `prompt_template_version`
   - `reference_file_ids`
3. Build a Responses API request using:
   - the prompt template
   - the variables
   - the reference images as `input_image` items by `file_id`
   - the `image_generation` tool

OpenAI documents that Responses accepts image inputs by file ID, and that image generation is provided through the `image_generation` tool.

### Example request shape

```json
{
  "model": "gpt-5",
  "prompt": {
    "id": "pmpt_123",
    "version": "3",
    "variables": {
      "exercise": "rack pull",
      "weight_lbs": "370"
    }
  },
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_image",
          "file_id": "file_ref_1"
        },
        {
          "type": "input_image",
          "file_id": "file_ref_2"
        },
        {
          "type": "input_image",
          "file_id": "file_ref_3"
        }
      ]
    }
  ],
  "tools": [
    {
      "type": "image_generation"
    }
  ]
}
```

Notes:

- Keep the model and tool configuration straightforward.
- Use the reference images in `input`.
- Keep the dynamic runtime fields in `prompt.variables`.

## Go example

Use the official OpenAI Go SDK style for the request, but the important part is the request shape.

```go
package primages

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Profile struct {
	PromptTemplateID      string
	PromptTemplateVersion string
	ReferenceFileIDs      []string
}

type GeneratePRImageInput struct {
	Exercise  string
	WeightLbs int
	Profile   Profile
}

func GeneratePRImage(ctx context.Context, apiKey string, in GeneratePRImageInput) ([]byte, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	content := make([]openai.ResponseInputMessageContentUnionParam, 0, len(in.Profile.ReferenceFileIDs))
	for _, fileID := range in.Profile.ReferenceFileIDs {
		content = append(content, openai.ResponseInputMessageContentUnionParam{
			OfInputImage: &openai.ResponseInputImageParam{
				Type:   openai.String("input_image"),
				FileID: openai.String(fileID),
			},
		})
	}

	resp, err := client.Responses.New(ctx, openai.ResponseNewParams{
		Model: openai.String("gpt-5"),
		Prompt: openai.ResponsePromptParam{
			ID: openai.String(in.Profile.PromptTemplateID),
			Variables: map[string]interface{}{
				"exercise":   in.Exercise,
				"weight_lbs": fmt.Sprintf("%d", in.WeightLbs),
			},
		},
		Input: []openai.ResponseInputItemUnionParam{
			{
				OfMessage: &openai.ResponseInputMessageParam{
					Role: openai.String("user"),
					Content: content,
				},
			},
		},
		Tools: []openai.ToolUnionParam{
			{
				OfImageGeneration: &openai.ImageGenerationToolParam{
					Type: openai.String("image_generation"),
				},
			},
	},
	})

	if err != nil {
		return nil, err
	}

	for _, output := range resp.Output {
		if output.Type == "image_generation_call" && output.Result != "" {
			data, err := base64.StdEncoding.DecodeString(output.Result)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}

	return nil, errors.New("no image returned from OpenAI")
}
```

If the exact generated SDK types differ in your installed version, keep the same request structure and adapt field names to the SDK version you have. The API capabilities themselves are documented here: prompt references, prompt variables, file-based image inputs, and image generation tools.

## Local CLI testing

Use curl locally first before wiring the Go code.

### 1) Upload one reference image

```bash
curl https://api.openai.com/v1/files \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F purpose="user_data" \
  -F file="@./references/character-sheet.png"
```

Copy the returned `id`.

### 2) Create one test generation request

```bash
curl https://api.openai.com/v1/responses \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "prompt": {
      "id": "pmpt_123",
      "version": "3",
      "variables": {
        "exercise": "rack pull",
        "weight_lbs": "370"
      }
    },
    "input": [
      {
        "role": "user",
        "content": [
          { "type": "input_image", "file_id": "file_ref_1" },
          { "type": "input_image", "file_id": "file_ref_2" },
          { "type": "input_image", "file_id": "file_ref_3" }
        ]
      }
    ],
    "tools": [
      { "type": "image_generation" }
    ]
  }'
```

Inspect the response and extract the generated image payload.

## Performance requirements

Do these two things:

### 1) Never re-upload the same references

Upload once, save `file_id`, reuse forever until you intentionally replace them. OpenAI's Files API is designed for reuse across endpoints.

### 2) Use prompt caching

Use a stable `prompt_cache_key` on requests for this feature. OpenAI documents `prompt_cache_key` on Responses requests and recommends putting repeated content at the beginning and dynamic content at the end. OpenAI also says prompt caching becomes available when prompts contain 1024 tokens or more.

Add this field to the request:

```json
"prompt_cache_key": "pr-image-yellow-train"
```

Use the same cache key for this profile.

## What not to do

Do not:

- re-upload the same PNGs on every request
- store the main logic in a giant conversation thread
- send only vague runtime text without the reference images
- rely on a previous response ID for standard one-shot PR generation

Use:

- file IDs
- one prompt template
- one runtime request
- one generated image

## Final implementation checklist

1. Upload the mascot/reference images with Files API.
2. Save all returned `file_ids` in DB.
3. Create one prompt template in OpenAI using the text above.
4. Save the prompt template ID and version in DB.
5. Build **POST /internal/pr-image**.
6. On request:
   - load the profile
   - send `exercise` and `weight_lbs` as prompt variables
   - attach the saved reference `file_ids` as `input_image`
   - call Responses API with `tools: [{type: "image_generation"}]`
   - Return the generated image bytes to the app.

That is the full implementation for what you asked for.

If you want, I can turn this into an even more copy-pasteable Cursor task file with folder structure and exact filenames.
