// Package livstid provides functionality for organizing and displaying photo albums.
package livstid

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/vertexai/genai"
)

// TagThumb specifies which thumbnail to use for AI tagging.
var TagThumb = "Album"

// AutoTag generates tags for an image using AI.
func AutoTag(ctx context.Context, model *genai.GenerativeModel, i *Image) ([]string, error) {
	thumb := i.Resize[TagThumb].Path
	bs, err := os.ReadFile(thumb)
	if err != nil {
		return nil, err
	}
	img := genai.ImageData("jpeg", bs)
	prompt := genai.Text("generate 1-5 comma-separated one-word tags. Here are some example tags: " +
		"bw for black and white photos, family for family photos, friends for friend photos, " +
		"landscape for landscape photos, motorcycle for motorcycle photos, nature for nature photos, " +
		"bird for bird photos, beach for beach photos, cycling for bicycling photos, " +
		"belgium for photos taken in Belgium. Tha tag animal should be included for photos of an animal " +
		"that is unlikely to be a pet. The tag forest should be used for forests, sunrise for sunrises. " +
		"Tags should be a present-tense singular word that a professional photographer would want to " +
		"organize their photo albums with. Use bw for blackandwhite. Do not combine multiple words. " +
		"Use urban for city photos. camping for camping photos. boat for boat photos. " +
		"photos with a bicycle in them should be tagged with bicycle. " +
		"photos that are taken in San Francisco should be tagged sf. " +
		"If you know the location of a photo, add the name of the place, city, or country as a tag. " +
		"If you know the animal genus, add the genus as a tag. do not use plural words. use rock instead of rocks.")
	resp, _ := model.GenerateContent(ctx, img, prompt)

	var tags []string

	for _, c := range resp.Candidates {
		p := c.Content.Parts[0]
		content := strings.ReplaceAll(fmt.Sprintf("%s", p), " ", "")
		tags = strings.Split(content, ",")
		break
	}

	return tags, nil
}
