// Package livstid provides functionality for organizing and displaying photo albums.
package livstid

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// TagThumb specifies which thumbnail to use for AI tagging.
var TagThumb = "Album"

// AutoTag generates tags for an image using AI.
func AutoTag(ctx context.Context, client *genai.Client, modelName string, i *Image) ([]string, error) {
	thumb := i.Resize[TagThumb].Path
	bs, err := os.ReadFile(thumb)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	imagePart := &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "image/jpeg",
			Data:     bs,
		},
	}

	textPart := &genai.Part{
		Text: fmt.Sprintf("generate 1-5 comma-separated one-word tags for this image titled %q (%s) at the path %q. Here are some example tags: "+
			"bw for black and white photos, family for family photos, friends for friend photos, "+
			"landscape for landscape photos, motorcycle for motorcycle photos, nature for nature photos, "+
			"sports for sports photos,"+
			"bird for bird photos, beach for beach photos, cycling for bicycling photos, "+
			"belgium for photos taken in Belgium. Tha tag animal should be included for photos of an animal "+
			"that is unlikely to be a pet. The tag forest should be used for forests, sunrise for sunrises. "+
			"Tags should be a present-tense singular word that a professional photographer would want to "+
			"organize their photo albums with. Use bw for blackandwhite. Do not combine multiple words. "+
			"Use urban for city photos. camping for camping photos. boat for boat photos. "+
			"photos with a bicycle in them should be tagged with bicycle. "+
			"photos that are taken in San Francisco should be tagged sf. "+
			"If you know the location of a photo, add the name of the place, city, or country as a tag. "+
			"If you know the animal genus, add the genus as a tag. do not use plural words. use rock instead of rocks.", i.Title, i.Description, i.RelPath),
	}

	// klog.Infof("prompt: %s", textPart)
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{imagePart, textPart},
			Role:  "user",
		},
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	var tags []string

	for _, c := range resp.Candidates {
		if len(c.Content.Parts) > 0 && c.Content.Parts[0].Text != "" {
			text := strings.TrimSpace(c.Content.Parts[0].Text)
			text = strings.ReplaceAll(text, ", ", ",")
			content := strings.ReplaceAll(text, " ", "_")
			//	klog.Infof("content: %s", content)
			tags = strings.Split(strings.ToLower(content), ",")
			break
		}
	}

	return tags, nil
}
